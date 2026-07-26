package rbac_test

import (
	"testing"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/rbac"
)

// newEvaluator 用生产环境同一个 glob 匹配器构造判定器（测试不走 wire）。
//
// 覆盖判定必须与 Casbin 模型的 globMatch 语义一致，因此这里注入基础设施的真实实现，
// 而不是自制 stub——否则测试通过而线上鉴权结论不同。
//
// 正因为要注入真实的 casbin 匹配器，本用例属于「领域规则 + 基础设施适配器」的集成测试，
// 故放在 infrastructure 层：这样 internal/domain 目录保持对 infrastructure 的零引用
// （见 docs/decisions.md D-017）。
func newEvaluator() *domain.PermissionEvaluator {
	return domain.NewPermissionEvaluator(infra.NewGlobPathMatcher())
}

func TestPermissionCovers(t *testing.T) {
	tests := []struct {
		name     string
		existRes string
		existAct string
		newRes   string
		newAct   string
		want     bool
	}{
		{
			name:     "same wildcard path and method",
			existRes: "/v1/common/**",
			existAct: "*",
			newRes:   "/v1/common/**",
			newAct:   "*",
			want:     true,
		},
		{
			name:     "method star covers GET",
			existRes: "/v1/common/**",
			existAct: "*",
			newRes:   "/v1/common/**",
			newAct:   "GET",
			want:     true,
		},
		{
			name:     "wildcard path covers concrete path",
			existRes: "/v1/admin/**",
			existAct: "*",
			newRes:   "/v1/admin/banner",
			newAct:   "DELETE",
			want:     true,
		},
		{
			name:     "different prefix not covered",
			existRes: "/v1/admin/**",
			existAct: "*",
			newRes:   "/v1/common/**",
			newAct:   "*",
			want:     false,
		},
		{
			name:     "GET does not cover POST",
			existRes: "/v1/admin/banner",
			existAct: "GET",
			newRes:   "/v1/admin/banner",
			newAct:   "POST",
			want:     false,
		},
		{
			name:     "GET does not cover star",
			existRes: "/v1/admin/banner",
			existAct: "GET",
			newRes:   "/v1/admin/banner",
			newAct:   "*",
			want:     false,
		},
	}

	evaluator := newEvaluator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluator.PermissionCovers(tt.existRes, tt.existAct, tt.newRes, tt.newAct)
			if got != tt.want {
				t.Fatalf("PermissionCovers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckPermissionsAlreadyOwned(t *testing.T) {
	owned := []domain.PermissionItem{
		{Resource: "/v1/common/**", Action: "*"},
	}
	targets := []domain.PermissionItem{
		{Resource: "/v1/common/**", Action: "GET"},
	}

	if err := newEvaluator().CheckAlreadyOwned(owned, targets); err != domain.ErrPermissionAlreadyOwned {
		t.Fatalf("expected ErrPermissionAlreadyOwned, got %v", err)
	}
}

func TestCheckInternalPermissionRedundancy(t *testing.T) {
	items := []domain.PermissionItem{
		{Resource: "/v1/admin/**", Action: "*"},
		{Resource: "/v1/admin/banner", Action: "GET"},
	}

	if err := newEvaluator().CheckInternalRedundancy(items); err != domain.ErrPermissionAlreadyOwned {
		t.Fatalf("expected ErrPermissionAlreadyOwned, got %v", err)
	}
}

func TestNormalizePermissionVariants(t *testing.T) {
	// 三种入口对「空列表」的处理不同，迁移后必须保持各自语义。
	if got, err := domain.NormalizeNewPermissions(nil); err != nil || got != nil {
		t.Fatalf("NormalizeNewPermissions(nil) = (%v, %v), want (nil, nil)", got, err)
	}

	if got, err := domain.NormalizeReplacementPermissions(nil); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("NormalizeReplacementPermissions(nil) = (%v, %v), want (empty slice, nil)", got, err)
	}

	if _, err := domain.CollectPermissions(nil); err != domain.ErrPermissionsRequired {
		t.Fatalf("CollectPermissions(nil) error = %v, want ErrPermissionsRequired", err)
	}

	if _, err := domain.CollectPermissions([]domain.PermissionItem{{Resource: " ", Action: "GET"}}); err != domain.ErrPermissionParamIncomplete {
		t.Fatalf("CollectPermissions(blank resource) error = %v, want ErrPermissionParamIncomplete", err)
	}
}
