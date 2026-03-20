package rbac_test

import (
	"testing"

	rbacerrors "f_ccg_video_back_ddd/internal/domain/rbac/domainerr"
	rbacservice "f_ccg_video_back_ddd/internal/domain/rbac/service"
	rbacvo "f_ccg_video_back_ddd/internal/domain/rbac/valueobject"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/rbac"
)

// newEvaluator 用生产环境的 glob 匹配器构造判定器，测试不经过 Wire。
//
// 权限覆盖判定必须使用与 Casbin 模型相同的 globMatch 语义；
// 该集成测试位于 infrastructure 层，domain 层不依赖具体匹配器。
func newEvaluator() *rbacservice.PermissionEvaluator {
	return rbacservice.NewPermissionEvaluator(infra.NewGlobPathMatcher())
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
	owned := []rbacvo.PermissionItem{
		{Resource: "/v1/common/**", Action: "*"},
	}
	targets := []rbacvo.PermissionItem{
		{Resource: "/v1/common/**", Action: "GET"},
	}

	if err := newEvaluator().CheckAlreadyOwned(owned, targets); err != rbacerrors.ErrPermissionAlreadyOwned {
		t.Fatalf("expected ErrPermissionAlreadyOwned, got %v", err)
	}
}

func TestCheckInternalPermissionRedundancy(t *testing.T) {
	items := []rbacvo.PermissionItem{
		{Resource: "/v1/admin/**", Action: "*"},
		{Resource: "/v1/admin/banner", Action: "GET"},
	}

	if err := newEvaluator().CheckInternalRedundancy(items); err != rbacerrors.ErrPermissionAlreadyOwned {
		t.Fatalf("expected ErrPermissionAlreadyOwned, got %v", err)
	}
}

func TestNormalizePermissionVariants(t *testing.T) {
	// 三种入口对空列表采用不同的约定，需分别验证。
	if got, err := rbacservice.NormalizeNewPermissions(nil); err != nil || got != nil {
		t.Fatalf("NormalizeNewPermissions(nil) = (%v, %v), want (nil, nil)", got, err)
	}

	if got, err := rbacservice.NormalizeReplacementPermissions(nil); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("NormalizeReplacementPermissions(nil) = (%v, %v), want (empty slice, nil)", got, err)
	}

	if _, err := rbacservice.CollectPermissions(nil); err != rbacerrors.ErrPermissionsRequired {
		t.Fatalf("CollectPermissions(nil) error = %v, want ErrPermissionsRequired", err)
	}

	if _, err := rbacservice.CollectPermissions([]rbacvo.PermissionItem{{Resource: " ", Action: "GET"}}); err != rbacerrors.ErrPermissionParamIncomplete {
		t.Fatalf("CollectPermissions(blank resource) error = %v, want ErrPermissionParamIncomplete", err)
	}
}
