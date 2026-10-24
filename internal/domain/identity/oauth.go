package identity

import (
	"fmt"
	"regexp"
	"strings"
)

// OAuthProfile 第三方登录带回的用户画像（值对象）。
//
// 字段与原 api/user_api/oauth_login_helper.go 的 oauthUserProfile 一一对应。
type OAuthProfile struct {
	// EmailVerified is an explicit provider assertion of email ownership.
	EmailVerified bool
	// RegisterSource 注册来源，同时作为 provider 名（qq/google/github/linuxdo/x）。
	RegisterSource string
	// OpenID 第三方用户唯一标识。
	OpenID string
	// Email 第三方返回的邮箱，可能为空。
	Email string
	// FallbackEmail 邮箱缺失时用于拼装兜底邮箱的本地部分（目前仅 linuxdo 使用其用户名）。
	FallbackEmail string
	// Avatar 第三方头像地址。
	Avatar string
	// RegisterIP 首次登录（注册）时的客户端 IP。
	RegisterIP string
	// RegisterAddr 首次登录（注册）时的 IP 归属地。
	RegisterAddr string
}

// fallbackEmailLocalPartPattern 兜底邮箱本地部分的非法字符集合。
var fallbackEmailLocalPartPattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// Normalize 去除首尾空白并校验必填项，返回规范化后的副本。
func (p OAuthProfile) Normalize() (OAuthProfile, error) {
	p.RegisterSource = strings.TrimSpace(p.RegisterSource)
	p.OpenID = strings.TrimSpace(p.OpenID)
	p.Email = strings.TrimSpace(p.Email)

	if p.RegisterSource == "" {
		return OAuthProfile{}, ErrOAuthRegisterSourceEmpty
	}

	if p.OpenID == "" {
		return OAuthProfile{}, ErrOAuthOpenIDEmpty
	}

	return p, nil
}

// ResolvedEmail 返回落库使用的邮箱：优先第三方返回值，其次兜底邮箱。
func (p OAuthProfile) ResolvedEmail() string {
	if !p.EmailVerified {
		return BuildOAuthFallbackEmail(p.RegisterSource, p.OpenID, p.FallbackEmail)
	}
	return NormalizeOAuthEmail(p.RegisterSource, p.OpenID, p.Email, p.FallbackEmail)
}

// SanitizeOAuthFallbackLocalPart 把第三方用户名规范为合法的邮箱本地部分。
//
// 等价原 api/user_api/oauth_login_helper.go 的 sanitizeOAuthFallbackLocalPart。
func SanitizeOAuthFallbackLocalPart(localPart string) string {
	localPart = strings.TrimSpace(localPart)
	localPart = fallbackEmailLocalPartPattern.ReplaceAllString(localPart, "_")
	localPart = strings.Trim(localPart, "._-")

	return localPart
}

// BuildOAuthFallbackEmail 生成兜底邮箱。
//
// linuxdo 优先用「规范化用户名@linuxdo」，其余（含 linuxdo 用户名为空时）
// 回落到「provider_identifier@oauth.local」。
func BuildOAuthFallbackEmail(provider, identifier, fallbackEmail string) string {
	if strings.EqualFold(strings.TrimSpace(provider), "linuxdo") {
		localPart := SanitizeOAuthFallbackLocalPart(fallbackEmail)
		if localPart != "" {
			return localPart + "@linuxdo"
		}
	}

	return fmt.Sprintf("%s_%s@oauth.local", provider, identifier)
}

// NormalizeOAuthEmail 第三方邮箱为空时生成兜底邮箱，否则原样返回。
func NormalizeOAuthEmail(provider, identifier, email, fallbackEmail string) string {
	email = strings.TrimSpace(email)
	if email != "" {
		return email
	}

	return BuildOAuthFallbackEmail(provider, identifier, fallbackEmail)
}
