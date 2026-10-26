package contentops

import (
	"context"
	"net/url"
	"strings"

	"f_ccg_video_back_ddd/internal/application/shared"

	"github.com/google/uuid"
)

// OAuth 提供商标识（写入 state 存储的 key 组成部分，属于外部契约）。
const (
	oauthProviderQQ      = "qq"
	oauthProviderGoogle  = "google"
	oauthProviderGitHub  = "github"
	oauthProviderLinuxDo = "linuxdo"
	oauthProviderX       = "x"
)

// GitHubURLParams GitHub 授权链接参数。
type GitHubURLParams struct {
	// CodeChallenge PKCE 校验串（可选）。
	CodeChallenge string
	// CodeChallengeMethod PKCE 算法（可选，默认 S256）。
	CodeChallengeMethod string
}

// XURLParams X（Twitter）授权链接参数。
type XURLParams struct {
	// CodeChallenge PKCE 校验串。
	CodeChallenge string
	// CodeChallengeMethod PKCE 算法（默认 S256）。
	CodeChallengeMethod string
}

// OAuthURLService 三方登录授权链接用例服务
// （原 api/site_api 的 qq_url.go / google_url.go / github_url.go / linuxdo_url.go / x_url.go）。
//
// Configuration is checked before generating and storing a one-time state.
type OAuthURLService struct {
	provider shared.ConfigReader
	states   OAuthStateStore
}

// NewOAuthURLService 构造三方登录授权链接服务。
func NewOAuthURLService(provider shared.ConfigReader, states OAuthStateStore) *OAuthURLService {
	return &OAuthURLService{provider: provider, states: states}
}

// QQURL generates a server-issued one-time state and QQ authorization URL.
func (s *OAuthURLService) QQURL(ctx context.Context) (string, error) {
	cfg := s.provider.Get()
	if cfg == nil || cfg.ThirdLogin == nil || cfg.ThirdLogin.QQ == nil || strings.TrimSpace(cfg.ThirdLogin.QQ.AppID) == "" || strings.TrimSpace(cfg.ThirdLogin.QQ.AppKey) == "" || strings.TrimSpace(cfg.ThirdLogin.QQ.Redirect) == "" {
		return "", nil
	}

	state, err := s.newState(ctx, oauthProviderQQ)
	if err != nil {
		return "", err
	}
	params := url.Values{}
	params.Set("state", state)
	params.Set("which", "Login")
	params.Set("display", "pc")
	params.Set("response_type", "code")
	params.Set("client_id", cfg.ThirdLogin.QQ.AppID)
	params.Set("redirect_uri", cfg.ThirdLogin.QQ.Redirect)

	return "https://graph.qq.com/oauth2.0/show?" + params.Encode(), nil
}

// GoogleURL 保存服务端生成的一次性 state 并生成 Google 授权链接。
func (s *OAuthURLService) GoogleURL(ctx context.Context) (string, error) {
	cfg := s.provider.Get()
	if cfg == nil || cfg.ThirdLogin == nil || cfg.ThirdLogin.Google == nil || strings.TrimSpace(cfg.ThirdLogin.Google.ClientID) == "" || strings.TrimSpace(cfg.ThirdLogin.Google.ClientSecret) == "" || strings.TrimSpace(cfg.ThirdLogin.Google.Redirect) == "" {
		return "", nil
	}
	state, err := s.newState(ctx, oauthProviderGoogle)
	if err != nil {
		return "", err
	}

	params := url.Values{}
	params.Set("client_id", cfg.ThirdLogin.Google.ClientID)
	params.Set("redirect_uri", cfg.ThirdLogin.Google.Redirect)
	params.Set("response_type", "code")
	params.Set("scope", "openid profile email")
	params.Set("prompt", "select_account")

	if trimmed := strings.TrimSpace(state); trimmed != "" {
		params.Set("state", trimmed)
	}

	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode(), nil
}

// GitHubURL 保存 state 并生成 GitHub 授权链接（支持 PKCE）。
func (s *OAuthURLService) GitHubURL(ctx context.Context, params GitHubURLParams) (string, error) {
	cfg := s.provider.Get()
	if cfg == nil || cfg.ThirdLogin == nil || cfg.ThirdLogin.GitHub == nil || strings.TrimSpace(cfg.ThirdLogin.GitHub.ClientID) == "" || strings.TrimSpace(cfg.ThirdLogin.GitHub.ClientSecret) == "" || strings.TrimSpace(cfg.ThirdLogin.GitHub.Redirect) == "" {
		return "", nil
	}
	state, err := s.newState(ctx, oauthProviderGitHub)
	if err != nil {
		return "", err
	}

	query := url.Values{}
	query.Set("client_id", cfg.ThirdLogin.GitHub.ClientID)
	query.Set("redirect_uri", cfg.ThirdLogin.GitHub.Redirect)
	query.Set("scope", "read:user user:email")

	query.Set("state", state)

	if challenge := strings.TrimSpace(params.CodeChallenge); challenge != "" {
		query.Set("code_challenge", challenge)

		method := strings.TrimSpace(params.CodeChallengeMethod)
		if method == "" {
			method = "S256"
		}

		query.Set("code_challenge_method", method)
	}

	return "https://github.com/login/oauth/authorize?" + query.Encode(), nil
}

// LinuxDoURL 保存 state 并生成 Linux.do 授权链接。
func (s *OAuthURLService) LinuxDoURL(ctx context.Context) (string, error) {
	cfg := s.provider.Get()
	if cfg == nil || cfg.ThirdLogin == nil || cfg.ThirdLogin.LinuxDo == nil || strings.TrimSpace(cfg.ThirdLogin.LinuxDo.ClientID) == "" || strings.TrimSpace(cfg.ThirdLogin.LinuxDo.ClientSecret) == "" || strings.TrimSpace(cfg.ThirdLogin.LinuxDo.Redirect) == "" {
		return "", nil
	}
	state, err := s.newState(ctx, oauthProviderLinuxDo)
	if err != nil {
		return "", err
	}

	query := url.Values{}
	query.Set("client_id", cfg.ThirdLogin.LinuxDo.ClientID)
	query.Set("redirect_uri", cfg.ThirdLogin.LinuxDo.Redirect)
	query.Set("response_type", "code")
	query.Set("scope", "user")

	if trimmed := strings.TrimSpace(state); trimmed != "" {
		query.Set("state", trimmed)
	}

	return "https://connect.linux.do/oauth2/authorize?" + query.Encode(), nil
}

// XURL 保存 state 并生成 X（Twitter）授权链接（强制 PKCE）。
func (s *OAuthURLService) XURL(ctx context.Context, params XURLParams) (string, error) {
	cfg := s.provider.Get()
	if cfg == nil || cfg.ThirdLogin == nil || cfg.ThirdLogin.X == nil || strings.TrimSpace(cfg.ThirdLogin.X.ClientID) == "" || strings.TrimSpace(cfg.ThirdLogin.X.ClientSecret) == "" || strings.TrimSpace(cfg.ThirdLogin.X.Redirect) == "" {
		return "", nil
	}
	state, err := s.newState(ctx, oauthProviderX)
	if err != nil {
		return "", err
	}

	query := url.Values{}
	query.Set("response_type", "code")
	query.Set("client_id", cfg.ThirdLogin.X.ClientID)
	query.Set("redirect_uri", cfg.ThirdLogin.X.Redirect)
	query.Set("scope", "users.read users.email")
	query.Set("code_challenge", strings.TrimSpace(params.CodeChallenge))

	method := strings.TrimSpace(params.CodeChallengeMethod)
	if method == "" {
		method = "S256"
	}

	query.Set("code_challenge_method", method)

	query.Set("state", state)

	return "https://x.com/i/oauth2/authorize?" + query.Encode(), nil
}

// newState always generates state on the server; caller-selected values are not trusted.
func (s *OAuthURLService) newState(ctx context.Context, provider string) (string, error) {
	state := uuid.NewString()
	if err := s.states.StoreOAuthState(ctx, provider, state); err != nil {
		return "", err
	}
	return state, nil
}
