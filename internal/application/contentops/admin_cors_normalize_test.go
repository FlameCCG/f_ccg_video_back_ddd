package contentops

import (
	"strings"
	"testing"
)

func TestNormalizeAdminCorsOrigins(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out, err := normalizeAdminCorsOrigins(nil)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if len(out) != 0 {
			t.Fatalf("expected empty, got %v", out)
		}
	})

	t.Run("star and urls", func(t *testing.T) {
		out, err := normalizeAdminCorsOrigins([]string{
			" * ",
			"https://www.example.com/path",
			"https://www.example.com",
			"http://localhost:3000",
			"",
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if len(out) != 3 {
			t.Fatalf("expected 3 unique origins, got %v", out)
		}

		if out[0] != "*" || out[1] != "https://www.example.com" || out[2] != "http://localhost:3000" {
			t.Fatalf("unexpected normalized list: %v", out)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := normalizeAdminCorsOrigins([]string{"ftp://x.com"})
		if err == nil {
			t.Fatal("expected error for ftp scheme")
		}

		if !strings.Contains(err.Error(), "scheme") && !strings.Contains(err.Error(), "无效") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
