package identity

import (
	"testing"

	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
)

// 兜底邮箱生成规则已上移到领域层（domain.NormalizeOAuthEmail），
// 原用例逐条迁移，断言值保持不变以守住外部契约。

func TestNormalizeOAuthEmail_ReturnsProvidedEmail(t *testing.T) {
	t.Parallel()

	email := domain.NormalizeOAuthEmail("linuxdo", "123", "user@example.com", "linuxdo-user")
	require.Equal(t, "user@example.com", email, "expected provided email preserved, got %s", email)
}

func TestNormalizeOAuthEmail_UsesLinuxDoUsernameFallback(t *testing.T) {
	t.Parallel()

	email := domain.NormalizeOAuthEmail("linuxdo", "232178", "", "linuxdo_user")
	require.Equal(t, "linuxdo_user@linuxdo", email, "expected linuxdo username fallback, got %s", email)
}

func TestNormalizeOAuthEmail_SanitizesLinuxDoUsernameFallback(t *testing.T) {
	t.Parallel()

	email := domain.NormalizeOAuthEmail("linuxdo", "232178", "", "linux do!?")
	require.Equal(t, "linux_do@linuxdo", email, "expected sanitized linuxdo fallback, got %s", email)
}

func TestNormalizeOAuthEmail_FallsBackToLegacyFormatWhenLinuxDoUsernameMissing(t *testing.T) {
	t.Parallel()

	email := domain.NormalizeOAuthEmail("linuxdo", "232178", "", "")
	require.Equal(t, "linuxdo_232178@oauth.local", email, "expected legacy fallback when username missing, got %s", email)
}

func TestNormalizeOAuthEmail_KeepsLegacyFormatForOtherProviders(t *testing.T) {
	t.Parallel()

	email := domain.NormalizeOAuthEmail("github", "12345", "", "octocat")
	require.Equal(t, "github_12345@oauth.local", email, "expected legacy fallback for non-linuxdo provider, got %s", email)
}
