package identity

import (
	"testing"

	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
)

// 验证 domain.NormalizeOAuthEmail 的邮箱回退规则与外部契约。

func TestNormalizeOAuthEmail_ReturnsProvidedEmail(t *testing.T) {
	email := (identitydomain.OAuthProfile{RegisterSource: "linuxdo", OpenID: "123", Email: "user@example.com", FallbackEmail: "linuxdo-user"}).StoredEmail()
	if email != "user@example.com" {
		t.Fatalf("expected provided email preserved, got %s", email)
	}
}

func TestNormalizeOAuthEmail_UsesLinuxDoUsernameFallback(t *testing.T) {
	email := (identitydomain.OAuthProfile{RegisterSource: "linuxdo", OpenID: "232178", Email: "", FallbackEmail: "linuxdo_user"}).StoredEmail()
	if email != "linuxdo_user@linuxdo" {
		t.Fatalf("expected linuxdo username fallback, got %s", email)
	}
}

func TestNormalizeOAuthEmail_SanitizesLinuxDoUsernameFallback(t *testing.T) {
	email := (identitydomain.OAuthProfile{RegisterSource: "linuxdo", OpenID: "232178", Email: "", FallbackEmail: "linux do!?"}).StoredEmail()
	if email != "linux_do@linuxdo" {
		t.Fatalf("expected sanitized linuxdo fallback, got %s", email)
	}
}

func TestNormalizeOAuthEmail_FallsBackToLegacyFormatWhenLinuxDoUsernameMissing(t *testing.T) {
	email := (identitydomain.OAuthProfile{RegisterSource: "linuxdo", OpenID: "232178", Email: "", FallbackEmail: ""}).StoredEmail()
	if email != "linuxdo_232178@oauth.local" {
		t.Fatalf("expected legacy fallback when username missing, got %s", email)
	}
}

func TestNormalizeOAuthEmail_KeepsLegacyFormatForOtherProviders(t *testing.T) {
	email := (identitydomain.OAuthProfile{RegisterSource: "github", OpenID: "12345", Email: "", FallbackEmail: "octocat"}).StoredEmail()
	if email != "github_12345@oauth.local" {
		t.Fatalf("expected legacy fallback for non-linuxdo provider, got %s", email)
	}
}
