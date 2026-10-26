package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"f_ccg_video_back_ddd/pkg/logctx"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	app "f_ccg_video_back_ddd/internal/application/identity"

	third_login "f_ccg_video_back_ddd/internal/config/thirdlogin"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"go.uber.org/zap"
)

//nolint:gosec // These constants are public OAuth endpoint URLs, not credentials.
const (
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
	// googleHTTPTimeout Google 端点单次请求超时（与原实现一致）。
	googleHTTPTimeout = 10 * time.Second
)

// GoogleAccessToken 表示 Google OAuth access token 响应。
type GoogleAccessToken struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	IDToken          string `json:"id_token"`
	Scope            string `json:"scope"`
	TokenType        string `json:"token_type"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// UserGoogleInfo 表示 Google OpenID Connect 用户资料。
type UserGoogleInfo struct {
	Sub           string `json:"sub"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Locale        string `json:"locale"`
}

// GoogleClient Google OAuth 客户端。
type GoogleClient struct {
	cfgProvider *infraconfig.Provider
	httpClient  *http.Client
	logger      *zap.Logger
}

var _ app.GoogleOAuthClient = (*GoogleClient)(nil)

// NewGoogleClient 构造 Google OAuth 客户端。
func NewGoogleClient(cfgProvider *infraconfig.Provider, logger *zap.Logger) *GoogleClient {
	return &GoogleClient{
		cfgProvider: cfgProvider,
		httpClient:  &http.Client{Timeout: googleHTTPTimeout},
		logger:      logger,
	}
}

// FetchUser 使用授权码换取 Google 用户信息，并归一化为通用画像。
func (c *GoogleClient) FetchUser(ctx context.Context, code string) (*app.OAuthUserInfo, error) {
	info, err := c.GetUserInfo(ctx, code)
	if err != nil {
		return nil, err
	}

	return &app.OAuthUserInfo{
		OpenID:        info.Sub,
		Email:         info.Email,
		EmailVerified: info.EmailVerified,
		Avatar:        info.Picture,
	}, nil
}

// GetUserInfo 使用授权码换取 Google 用户信息。
func (c *GoogleClient) GetUserInfo(ctx context.Context, code string) (*UserGoogleInfo, error) {
	accessToken, err := c.accessToken(ctx, code)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken.AccessToken)
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google userinfo request failed, status=%d", resp.StatusCode)
	}

	logctx.With(ctx, c.logger).Debug("Google UserInfo Response", zap.String("event", "external.google.get_user_info.diagnostic"), zap.String("operation", "external.google.get_user_info"), zap.Int("status", resp.StatusCode))

	var userInfo UserGoogleInfo
	if err = json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	if userInfo.Sub == "" {
		return nil, errors.New("google user sub is empty")
	}

	return &userInfo, nil
}

func (c *GoogleClient) accessToken(ctx context.Context, code string) (*GoogleAccessToken, error) {
	googleConf, err := c.conf()
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", googleConf.ClientID)
	form.Set("client_secret", googleConf.ClientSecret)
	form.Set("redirect_uri", googleConf.Redirect)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(form.Encode()))
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

	logctx.With(ctx, c.logger).Debug("Google Token Response", zap.String("event", "external.google.access_token.diagnostic"), zap.String("operation", "external.google.access_token"), zap.Int("status", resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google token request failed, status=%d", resp.StatusCode)
	}

	var accessToken GoogleAccessToken
	if err = json.Unmarshal(body, &accessToken); err != nil {
		return nil, err
	}

	if accessToken.Error != "" {
		return nil, fmt.Errorf("google token request failed: %s %s", accessToken.Error, accessToken.ErrorDescription)
	}

	if accessToken.AccessToken == "" {
		return nil, errors.New("google access token is empty")
	}

	return &accessToken, nil
}

func (c *GoogleClient) conf() (*third_login.Google, error) {
	cfg := c.cfgProvider.Get()
	if cfg == nil || cfg.ThirdLogin == nil || cfg.ThirdLogin.Google == nil {
		return nil, errors.New("google auth config is missing")
	}

	return cfg.ThirdLogin.Google, nil
}
