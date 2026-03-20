package identity

import (
	"testing"

	identityvo "f_ccg_video_back_ddd/internal/domain/identity/valueobject"
)

// 验证 domain.NormalizeOAuthEmail 的邮箱回退规则与外部契约。

func TestNormalizeOAuthEmail_ReturnsProvidedEmail(t *testing.T) {
	email := identityvo.NormalizeOAuthEmail("linuxdo", "123", "user@example.com", "linuxdo-user")
	if email != "user@example.com" {
		t.Fatalf("expected provided email preserved, got %s", email)
	}
}

func TestNormalizeOAuthEmail_UsesLinuxDoUsernameFallback(t *testing.T) {
	email := identityvo.NormalizeOAuthEmail("linuxdo", "232178", "", "linuxdo_user")
	if email != "linuxdo_user@linuxdo" {
		t.Fatalf("expected linuxdo username fallback, got %s", email)
	}
}

func TestNormalizeOAuthEmail_SanitizesLinuxDoUsernameFallback(t *testing.T) {
	email := identityvo.NormalizeOAuthEmail("linuxdo", "232178", "", "linux do!?")
	if email != "linux_do@linuxdo" {
		t.Fatalf("expected sanitized linuxdo fallback, got %s", email)
	}
}

func TestNormalizeOAuthEmail_FallsBackToLegacyFormatWhenLinuxDoUsernameMissing(t *testing.T) {
	email := identityvo.NormalizeOAuthEmail("linuxdo", "232178", "", "")
	if email != "linuxdo_232178@oauth.local" {
		t.Fatalf("expected legacy fallback when username missing, got %s", email)
	}
}

func TestNormalizeOAuthEmail_KeepsLegacyFormatForOtherProviders(t *testing.T) {
	email := identityvo.NormalizeOAuthEmail("github", "12345", "", "octocat")
	if email != "github_12345@oauth.local" {
		t.Fatalf("expected legacy fallback for non-linuxdo provider, got %s", email)
	}
}
