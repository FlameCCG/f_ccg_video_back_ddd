package identity

import (
	"fmt"
	"regexp"
	"strings"
)

// OAuthProfile 第三方登录返回的用户画像值对象，命令字段保持公开。
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

// ResolvedEmail 返回落库使用的邮箱：未验证时只用兜底邮箱，已验证时优先第三方返回值。
func (p OAuthProfile) ResolvedEmail() string {
	if !p.EmailVerified {
		return p.fallbackAddress()
	}

	return p.StoredEmail()
}

// StoredEmail 在邮箱非空时原样保留，否则生成兜底邮箱。不查看 EmailVerified。
func (p OAuthProfile) StoredEmail() string {
	email := strings.TrimSpace(p.Email)
	if email != "" {
		return email
	}

	return p.fallbackAddress()
}

// CanClaimExistingEmail 只有提供方明确验证过的非空邮箱才能关联既有账号。
func (p OAuthProfile) CanClaimExistingEmail() bool {
	return p.EmailVerified && strings.TrimSpace(p.Email) != ""
}

func (p OAuthProfile) fallbackAddress() string {
	if strings.EqualFold(strings.TrimSpace(p.RegisterSource), "linuxdo") {
		localPart := p.sanitizeLocalPart(p.FallbackEmail)
		if localPart != "" {
			return localPart + "@linuxdo"
		}
	}

	return fmt.Sprintf("%s_%s@oauth.local", p.RegisterSource, p.OpenID)
}

func (p OAuthProfile) sanitizeLocalPart(localPart string) string {
	localPart = strings.TrimSpace(localPart)
	localPart = fallbackEmailLocalPartPattern.ReplaceAllString(localPart, "_")
	localPart = strings.Trim(localPart, "._-")

	return localPart
}
