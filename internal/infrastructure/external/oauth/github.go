package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"f_ccg_video_back_ddd/pkg/apperr"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	app "f_ccg_video_back_ddd/internal/application/identity"
	third_login "f_ccg_video_back_ddd/internal/config/thirdlogin"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
)

//nolint:gosec // These constants are public OAuth endpoint URLs, not credentials.
const (
	githubAuthorizeTokenURL = "https://github.com/login/oauth/access_token"
	githubUserInfoURL       = "https://api.github.com/user"
	githubUserEmailsURL     = "https://api.github.com/user/emails"
	githubAPIVersion        = "2022-11-28"
	// githubHTTPTimeout GitHub 端点单次请求超时（与原实现一致）。
	githubHTTPTimeout = 10 * time.Second
)

// GitHubAccessToken 表示 GitHub OAuth access token 响应。
type GitHubAccessToken struct {
	AccessToken      string `json:"access_token"`
	Scope            string `json:"scope"`
	TokenType        string `json:"token_type"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	ErrorURI         string `json:"error_uri"`
}

// UserGitHubInfo 表示 GitHub 用户基础资料。
type UserGitHubInfo struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
}

// UserEmail 表示 GitHub 用户邮箱条目。
type UserEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// GitHubClient GitHub OAuth 客户端。
type GitHubClient struct {
	cfgProvider *infraconfig.Provider
	httpClient  *http.Client
}

var _ app.GitHubOAuthClient = (*GitHubClient)(nil)

// NewGitHubClient 构造 GitHub OAuth 客户端。
func NewGitHubClient(cfgProvider *infraconfig.Provider) *GitHubClient {
	return &GitHubClient{
		cfgProvider: cfgProvider,
		httpClient:  &http.Client{Timeout: githubHTTPTimeout},
	}
}

// FetchUser 使用授权码换取 GitHub 用户信息，并归一化为通用画像。
func (c *GitHubClient) FetchUser(ctx context.Context, code, codeVerifier string) (*app.OAuthUserInfo, error) {
	info, err := c.GetUserInfo(ctx, code, codeVerifier)
	if err != nil {
		return nil, err
	}

	return &app.OAuthUserInfo{
		OpenID:        strconv.FormatInt(info.ID, 10),
		Email:         info.Email,
		EmailVerified: strings.TrimSpace(info.Email) != "",
		Avatar:        info.AvatarURL,
	}, nil
}

// GetUserInfo 使用授权码换取 GitHub 用户信息；邮箱为空时补查 /user/emails。
func (c *GitHubClient) GetUserInfo(ctx context.Context, code, codeVerifier string) (*UserGitHubInfo, error) {
	accessToken, err := c.accessToken(ctx, code, codeVerifier)
	if err != nil {
		return nil, err
	}

	resp, err := DoWithRetry(c.httpClient, func() (*http.Request, error) {
		return newGitHubRequest(ctx, http.MethodGet, githubUserInfoURL, accessToken.AccessToken)
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, apperr.Private("oauth.provider", fmt.Errorf("github userinfo request failed, status=%d body=%s", resp.StatusCode, string(body)))
	}

	var userInfo UserGitHubInfo
	if err = json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	if userInfo.ID == 0 {
		return nil, errors.New("github user id is empty")
	}

	// /user does not attest email ownership. Only /user/emails verified entries are trusted.
	userInfo.Email = ""
	if email, emailErr := c.primaryEmail(ctx, accessToken.AccessToken); emailErr == nil {
		userInfo.Email = email
	}

	return &userInfo, nil
}

func (c *GitHubClient) accessToken(ctx context.Context, code, codeVerifier string) (*GitHubAccessToken, error) {
	githubConf, err := c.conf()
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("client_id", githubConf.ClientID)
	form.Set("client_secret", githubConf.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", githubConf.Redirect)

	if strings.TrimSpace(codeVerifier) != "" {
		form.Set("code_verifier", codeVerifier)
	}

	formBody := form.Encode()

	resp, err := DoWithRetry(c.httpClient, func() (*http.Request, error) {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, githubAuthorizeTokenURL, strings.NewReader(formBody))
		if reqErr != nil {
			return nil, reqErr
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		return req, nil
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, apperr.Private("oauth.provider", fmt.Errorf("github token request failed, status=%d body=%s", resp.StatusCode, string(body)))
	}

	var accessToken GitHubAccessToken
	if err = json.Unmarshal(body, &accessToken); err != nil {
		return nil, err
	}

	if accessToken.Error != "" {
		return nil, fmt.Errorf("github token request failed: %s %s", accessToken.Error, accessToken.ErrorDescription)
	}

	if accessToken.AccessToken == "" {
		return nil, errors.New("github access token is empty")
	}

	return &accessToken, nil
}

// primaryEmail returns verified addresses only, preferring the primary address.
func (c *GitHubClient) primaryEmail(ctx context.Context, accessToken string) (string, error) {
	resp, err := DoWithRetry(c.httpClient, func() (*http.Request, error) {
		return newGitHubRequest(ctx, http.MethodGet, githubUserEmailsURL, accessToken)
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", apperr.Private("oauth.provider", fmt.Errorf("github emails request failed, status=%d body=%s", resp.StatusCode, string(body)))
	}

	var emails []UserEmail
	if err = json.Unmarshal(body, &emails); err != nil {
		return "", err
	}

	for _, item := range emails {
		if item.Primary && item.Verified && strings.TrimSpace(item.Email) != "" {
			return item.Email, nil
		}
	}

	for _, item := range emails {
		if item.Verified && strings.TrimSpace(item.Email) != "" {
			return item.Email, nil
		}
	}

	return "", nil
}

func (c *GitHubClient) conf() (*third_login.GitHub, error) {
	cfg := c.cfgProvider.Get()
	if cfg == nil || cfg.ThirdLogin == nil || cfg.ThirdLogin.GitHub == nil {
		return nil, errors.New("github auth config is missing")
	}

	return cfg.ThirdLogin.GitHub, nil
}

func newGitHubRequest(ctx context.Context, method, requestURL, accessToken string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, requestURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)

	return req, nil
}
