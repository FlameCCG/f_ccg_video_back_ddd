package rbac_test

import (
	"errors"
	"strings"
	"testing"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"
)

func TestValidateMenuIconAcceptsSafeIcons(t *testing.T) {
	for _, icon := range []string{
		"",
		"icon-class-name", // 手工路径允许非 SVG 图标（字体图标类名等）
		"<svg viewBox=\"0 0 24 24\"></svg>",
		"<svg><path d=\"M0 0h24v24H0z\"/></svg>",
		"data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=", // base64 data URI：原文不含危险 token，放行
		strings.Repeat("a", 2048),                    // 恰好上限
	} {
		if err := domain.ValidateMenuIcon(icon); err != nil {
			t.Errorf("ValidateMenuIcon(%q) = %v, want nil", icon, err)
		}
	}
}

func TestValidateMenuIconRejectsUnsafeIcons(t *testing.T) {
	for _, icon := range []string{
		"<script>alert(1)</script>",
		"<svg><script>alert(1)</script></svg>",
		"<SCRIPT>alert(1)</SCRIPT>", // 大小写变体
		"<svg><foreignObject><iframe src=x/></foreignObject></svg>",
		"<svg><foreignObject></foreignObject></svg>",
		"<svg onload=\"alert(1)\"></svg>",
		"<svg OnLoad=alert(1)></svg>", // 事件属性大小写变体
		"<svg onmouseover=\"alert(1)\"></svg>",
		"<a href=\"javascript:alert(1)\">x</a>",
		"javascript:alert(1)",
	} {
		if err := domain.ValidateMenuIcon(icon); !errors.Is(err, domain.ErrMenuIconUnsafe) {
			t.Errorf("ValidateMenuIcon(%q) = %v, want ErrMenuIconUnsafe", icon, err)
		}
	}
}

func TestValidateMenuIconRejectsTooLong(t *testing.T) {
	icon := strings.Repeat("a", 2049)
	if err := domain.ValidateMenuIcon(icon); !errors.Is(err, domain.ErrMenuIconTooLong) {
		t.Errorf("ValidateMenuIcon(2049 bytes) = %v, want ErrMenuIconTooLong", err)
	}
}
