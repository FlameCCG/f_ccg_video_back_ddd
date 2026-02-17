package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
	third_login "f_ccg_video_back_ddd/internal/infrastructure/config/thirdlogin"
)

const (
	linuxDoTokenURL    = "https://connect.linux.do/oauth2/token"
	linuxDoUserInfoURL = "https://connect.linux.do/api/user"
	linuxDoBaseURL     = "https://connect.linux.do"
	// linuxDoHTTPTimeout LinuxDo 首次建连在部分网络环境下明显慢于其他 OAuth 提供商，
	// 这里使用更长的单次超时，避免 10s * 2 的重复重试把慢响应误判成失败。
	linuxDoHTTPTimeout = 20 * time.Second
)

// linuxDoAccessToken 表示 Linux.do OAuth access token 响应。
type linuxDoAccessToken struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
	ExpiresIn        int    `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// rawUserLinuxDoInfo Linux.do 用户接口的原始响应（id 可能是数字或字符串）。
type rawUserLinuxDoInfo struct {
	ID             json.RawMessage `json:"id"`
	Username       string          `json:"username"`
	Name           string          `json:"name"`
	AvatarTemplate string          `json:"avatar_template"`
}

// UserLinuxDoInfo 表示 Linux.do 用户资料。
type UserLinuxDoInfo struct {
	ID             string
	Username       string
	Name           string
	AvatarTemplate string
	AvatarURL      string
}

// LinuxDoClient Linux.do OAuth 客户端。
type LinuxDoClient struct {
	cfgProvider *config.Provider
	httpClient  *http.Client
}

var _ app.LinuxDoOAuthClient = (*LinuxDoClient)(nil)

// NewLinuxDoClient 构造 Linux.do OAuth 客户端。
func NewLinuxDoClient(cfgProvider *config.Provider) *LinuxDoClient {
	return &LinuxDoClient{
		cfgProvider: cfgProvider,
		httpClient:  &http.Client{Timeout: linuxDoHTTPTimeout},
	}
}

// FetchUser 使用授权码换取 Linux.do 用户信息，并归一化为通用画像。
//
// Linux.do 不返回邮箱，用户名作为兜底邮箱的本地部分。
func (c *LinuxDoClient) FetchUser(ctx context.Context, code string) (*app.OAuthUserInfo, error) {
	info, err := c.GetUserInfo(ctx, code)
	if err != nil {
		return nil, err
	}

	return &app.OAuthUserInfo{
		OpenID:        info.ID,
		FallbackEmail: info.Username,
		Avatar:        info.AvatarURL,
	}, nil
}

// GetUserInfo 使用授权码换取 Linux.do 用户信息。
func (c *LinuxDoClient) GetUserInfo(ctx context.Context, code string) (*UserLinuxDoInfo, error) {
	token, err := c.accessToken(ctx, code)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, linuxDoUserInfoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("linuxdo userinfo request failed, status=%d body=%s", resp.StatusCode, string(body))
	}

	var raw rawUserLinuxDoInfo
	if err = json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	userID, err := normalizeLinuxDoUserID(raw.ID)
	if err != nil {
		return nil, err
	}

	return &UserLinuxDoInfo{
		ID:             userID,
		Username:       raw.Username,
		Name:           raw.Name,
		AvatarTemplate: raw.AvatarTemplate,
		AvatarURL:      buildLinuxDoAvatarURL(raw.AvatarTemplate),
	}, nil
}

func (c *LinuxDoClient) accessToken(ctx context.Context, code string) (*linuxDoAccessToken, error) {
	linuxDoConf, err := c.conf()
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("client_id", linuxDoConf.ClientID)
	form.Set("client_secret", linuxDoConf.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", linuxDoConf.Redirect)
	form.Set("grant_type", "authorization_code")
	formBody := form.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, linuxDoTokenURL, strings.NewReader(formBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("linuxdo token request failed, status=%d body=%s", resp.StatusCode, string(body))
	}

	var token linuxDoAccessToken
	if err = json.Unmarshal(body, &token); err != nil {
		return nil, err
	}

	if token.Error != "" {
		return nil, fmt.Errorf("linuxdo token request failed: %s %s", token.Error, token.ErrorDescription)
	}

	if strings.TrimSpace(token.AccessToken) == "" {
		return nil, errors.New("linuxdo access token is empty")
	}

	return &token, nil
}

func (c *LinuxDoClient) conf() (*third_login.LinuxDo, error) {
	cfg := c.cfgProvider.Get()
	if cfg == nil || cfg.ThirdLogin == nil || cfg.ThirdLogin.LinuxDo == nil {
		return nil, errors.New("linuxdo auth config is missing")
	}

	return cfg.ThirdLogin.LinuxDo, nil
}

// normalizeLinuxDoUserID 兼容字符串与数字两种 id 表示。
func normalizeLinuxDoUserID(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", errors.New("linuxdo user id is empty")
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString != "" {
			return asString, nil
		}
	}

	var asNumber json.Number
	if err := json.Unmarshal(raw, &asNumber); err == nil {
		asNumberString := strings.TrimSpace(asNumber.String())
		if asNumberString != "" {
			return asNumberString, nil
		}
	}

	return "", errors.New("linuxdo user id is invalid")
}

// buildLinuxDoAvatarURL 把头像模板补全为绝对地址（{size} 固定取 240）。
func buildLinuxDoAvatarURL(avatarTemplate string) string {
	avatarTemplate = strings.TrimSpace(avatarTemplate)
	if avatarTemplate == "" {
		return ""
	}

	avatarTemplate = strings.ReplaceAll(avatarTemplate, "{size}", "240")

	switch {
	case strings.HasPrefix(avatarTemplate, "http://"), strings.HasPrefix(avatarTemplate, "https://"):
		return avatarTemplate
	case strings.HasPrefix(avatarTemplate, "//"):
		return "https:" + avatarTemplate
	case strings.HasPrefix(avatarTemplate, "/"):
		return linuxDoBaseURL + avatarTemplate
	default:
		return linuxDoBaseURL + "/" + strings.TrimPrefix(avatarTemplate, "/")
	}
}
