package oauth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	app "f_ccg_video_back_ddd/internal/application/identity"
	third_login "f_ccg_video_back_ddd/internal/config/thirdlogin"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
)

const (
	xTokenURL    = "https://api.x.com/2/oauth2/token"
	xUserInfoURL = "https://api.x.com/2/users/me"
	xUserAgent   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"
	// xHTTPTimeout X 的 OAuth 端点对 Go 默认请求特征兼容性较差，
	// 使用更长的单次超时避免 10s * 2 的超时放大。
	xHTTPTimeout = 20 * time.Second
)

// XAccessToken 表示 X OAuth access token 响应。
type XAccessToken struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
}

// UserXInfo 表示 X 用户资料。
type UserXInfo struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Username        string `json:"username"`
	ProfileImageURL string `json:"profile_image_url"`
	ConfirmedEmail  string `json:"confirmed_email"`
}

// XUserInfoResponse 表示 X 用户信息 API 响应包装。
type XUserInfoResponse struct {
	Data UserXInfo `json:"data"`
}

// XClient X (Twitter) OAuth 客户端。
type XClient struct {
	cfgProvider *infraconfig.Provider
	httpClient  *http.Client
}

var _ app.XOAuthClient = (*XClient)(nil)

// NewXClient 构造 X OAuth 客户端。
func NewXClient(cfgProvider *infraconfig.Provider) *XClient {
	return &XClient{
		cfgProvider: cfgProvider,
		httpClient:  &http.Client{Timeout: xHTTPTimeout},
	}
}

// FetchUser 使用授权码换取 X 用户信息，并归一化为通用画像。
func (c *XClient) FetchUser(ctx context.Context, code, codeVerifier string) (*app.OAuthUserInfo, error) {
	info, err := c.GetUserInfo(ctx, code, codeVerifier)
	if err != nil {
		return nil, err
	}

	return &app.OAuthUserInfo{
		OpenID: info.ID,
		Email:  info.ConfirmedEmail,
		Avatar: info.ProfileImageURL,
	}, nil
}

// GetUserInfo 使用授权码换取 X 用户信息。
//
// 优先请求 confirmed_email 字段；若应用未授权该字段（403）则退化为不含邮箱的请求。
func (c *XClient) GetUserInfo(ctx context.Context, code, codeVerifier string) (*UserXInfo, error) {
	accessToken, err := c.accessToken(ctx, code, codeVerifier)
	if err != nil {
		return nil, err
	}

	userInfo, err := c.userInfoByFields(ctx, accessToken.AccessToken, "profile_image_url,confirmed_email")
	if err == nil {
		return userInfo, nil
	}

	if !strings.Contains(err.Error(), "status=403") {
		return nil, err
	}

	return c.userInfoByFields(ctx, accessToken.AccessToken, "profile_image_url")
}

func (c *XClient) userInfoByFields(ctx context.Context, accessToken, userFields string) (*UserXInfo, error) {
	params := url.Values{}
	params.Set("user.fields", userFields)
	requestURL := xUserInfoURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", xUserAgent)

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
		return nil, fmt.Errorf("x userinfo request failed, status=%d body=%s", resp.StatusCode, string(body))
	}

	var userInfoResp XUserInfoResponse
	if err = json.Unmarshal(body, &userInfoResp); err != nil {
		return nil, err
	}

	if strings.TrimSpace(userInfoResp.Data.ID) == "" {
		return nil, errors.New("x user id is empty")
	}

	return &userInfoResp.Data, nil
}

func (c *XClient) accessToken(ctx context.Context, code, codeVerifier string) (*XAccessToken, error) {
	xConf, err := c.conf()
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(codeVerifier) == "" {
		return nil, errors.New("x code verifier is empty")
	}

	form := url.Values{}
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", xConf.Redirect)
	form.Set("code_verifier", codeVerifier)
	form.Set("client_id", xConf.ClientID)
	formBody := form.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, xTokenURL, strings.NewReader(formBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", xUserAgent)

	if strings.TrimSpace(xConf.ClientSecret) != "" {
		req.SetBasicAuth(xConf.ClientID, xConf.ClientSecret)
	}

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
		return nil, fmt.Errorf("x token request failed, status=%d body=%s", resp.StatusCode, string(body))
	}

	body = normalizeXTokenBody(body)

	var accessToken XAccessToken
	if err = json.Unmarshal(body, &accessToken); err != nil {
		return nil, err
	}

	if accessToken.AccessToken == "" {
		return nil, errors.New("x access token is empty")
	}

	return &accessToken, nil
}

func (c *XClient) conf() (*third_login.X, error) {
	cfg := c.cfgProvider.Get()
	if cfg == nil || cfg.ThirdLogin == nil || cfg.ThirdLogin.X == nil {
		return nil, errors.New("x auth config is missing")
	}

	return cfg.ThirdLogin.X, nil
}

// normalizeXTokenBody 兼容 X 偶发返回 base64 编码 JSON 的情况。
func normalizeXTokenBody(body []byte) []byte {
	trimmed := bytes.TrimSpace(body)
	if json.Valid(trimmed) {
		return trimmed
	}

	if decoded, err := base64.RawURLEncoding.DecodeString(string(trimmed)); err == nil && json.Valid(decoded) {
		return decoded
	}

	if decoded, err := base64.StdEncoding.DecodeString(string(trimmed)); err == nil && json.Valid(decoded) {
		return decoded
	}

	return trimmed
}
