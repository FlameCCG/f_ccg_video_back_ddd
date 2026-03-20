package identity

import (
	"context"
)

// OAuthStateStore 一次性 OAuth state 存储端口。
//
// 实现见 internal/infrastructure/external/oauth.StateStore：Redis 优先、
// 进程内内存兜底，key 形如 oauth:state:{provider}:{state}，TTL 10 分钟。
type OAuthStateStore interface {
	// Store 保存一次性 state。
	Store(ctx context.Context, provider, state string) error
	// Consume 校验并消费 state；无效/过期/已消费时返回错误。
	Consume(ctx context.Context, provider, state string) error
}

// OAuthUserInfo 第三方用户信息（各 provider 适配器归一化后的结果）。
type OAuthUserInfo struct {
	// EmailVerified is an explicit provider assertion of email ownership.
	EmailVerified bool
	// OpenID 第三方用户唯一标识。
	OpenID string
	// Email 第三方返回的邮箱，可能为空。
	Email string
	// FallbackEmail 邮箱缺失时用于拼装兜底邮箱的本地部分（linuxdo 传其用户名）。
	FallbackEmail string
	// Avatar 第三方头像地址。
	Avatar string
}

// QQOAuthClient QQ 授权码换用户信息端口。
type QQOAuthClient interface {
	FetchUser(ctx context.Context, code string) (*OAuthUserInfo, error)
}

// GoogleOAuthClient Google 授权码换用户信息端口。
type GoogleOAuthClient interface {
	FetchUser(ctx context.Context, code string) (*OAuthUserInfo, error)
}

// GitHubOAuthClient GitHub 授权码换用户信息端口（支持 PKCE code_verifier）。
type GitHubOAuthClient interface {
	FetchUser(ctx context.Context, code, codeVerifier string) (*OAuthUserInfo, error)
}

// LinuxDoOAuthClient Linux.do 授权码换用户信息端口。
type LinuxDoOAuthClient interface {
	FetchUser(ctx context.Context, code string) (*OAuthUserInfo, error)
}

// XOAuthClient X (Twitter) 授权码换用户信息端口（强制 PKCE）。
type XOAuthClient interface {
	FetchUser(ctx context.Context, code, codeVerifier string) (*OAuthUserInfo, error)
}
