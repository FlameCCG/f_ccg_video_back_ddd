package contentops

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAdminCorsOrigins(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		out, err := normalizeAdminCorsOrigins(nil)
		require.NoError(t, err, "unexpected err: %v", err)

		require.Empty(t, out, "expected empty, got %v", out)
	})

	t.Run("star and urls", func(t *testing.T) {
		t.Parallel()

		out, err := normalizeAdminCorsOrigins([]string{
			" * ",
			"https://www.example.com/path",
			"https://www.example.com",
			"http://localhost:3000",
			"",
		})
		require.NoError(t, err, "unexpected err: %v", err)

		require.Len(t, out, 3, "expected 3 unique origins, got %v", out)

		require.Equal(t, "*", out[0], "unexpected normalized list: %v", out)
		require.Equal(t, "https://www.example.com", out[1], "unexpected normalized list: %v", out)
		require.Equal(t, "http://localhost:3000", out[2], "unexpected normalized list: %v", out)
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		_, err := normalizeAdminCorsOrigins([]string{"ftp://x.com"})
		require.Error(t, err, "expected error for ftp scheme")

		require.False(t, !strings.Contains(err.Error(), "scheme") && !strings.Contains(err.Error(), "无效"), "unexpected error: %v", err)
	})
}
