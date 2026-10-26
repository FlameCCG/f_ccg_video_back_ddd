package rbac_test

import (
	"testing"

	"github.com/stretchr/testify/require"

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
const (
	testV1AdminBanner = "/v1/admin/banner"
	testV1Common      = "/v1/common/**"
	testGET           = "GET"
)

func newEvaluator() *domain.PermissionEvaluator {
	return domain.NewPermissionEvaluator(infra.NewGlobPathMatcher())
}

func TestPermissionCovers(t *testing.T) {
	t.Parallel()

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
			existRes: testV1Common,
			existAct: "*",
			newRes:   testV1Common,
			newAct:   "*",
			want:     true,
		},
		{
			name:     "method star covers GET",
			existRes: testV1Common,
			existAct: "*",
			newRes:   testV1Common,
			newAct:   testGET,
			want:     true,
		},
		{
			name:     "wildcard path covers concrete path",
			existRes: "/v1/admin/**",
			existAct: "*",
			newRes:   testV1AdminBanner,
			newAct:   "DELETE",
			want:     true,
		},
		{
			name:     "different prefix not covered",
			existRes: "/v1/admin/**",
			existAct: "*",
			newRes:   testV1Common,
			newAct:   "*",
			want:     false,
		},
		{
			name:     "GET does not cover POST",
			existRes: testV1AdminBanner,
			existAct: testGET,
			newRes:   testV1AdminBanner,
			newAct:   "POST",
			want:     false,
		},
		{
			name:     "GET does not cover star",
			existRes: testV1AdminBanner,
			existAct: testGET,
			newRes:   testV1AdminBanner,
			newAct:   "*",
			want:     false,
		},
	}

	evaluator := newEvaluator()

	//nolint:paralleltest // Cases share parent fixtures or must finish before their parent resource cleanup.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluator.PermissionCovers(tt.existRes, tt.existAct, tt.newRes, tt.newAct)
			require.Equal(t, tt.want, got, "PermissionCovers() = %v, want %v", got, tt.want)
		})
	}
}

func TestCheckPermissionsAlreadyOwned(t *testing.T) {
	t.Parallel()

	owned := []domain.PermissionItem{
		{Resource: testV1Common, Action: "*"},
	}
	targets := []domain.PermissionItem{
		{Resource: testV1Common, Action: testGET},
	}

	{
		err := newEvaluator().CheckAlreadyOwned(owned, targets)
		require.ErrorIs(t, err, domain.ErrPermissionAlreadyOwned, "expected ErrPermissionAlreadyOwned, got %v", err)
	}
}

func TestCheckInternalPermissionRedundancy(t *testing.T) {
	t.Parallel()

	items := []domain.PermissionItem{
		{Resource: "/v1/admin/**", Action: "*"},
		{Resource: testV1AdminBanner, Action: testGET},
	}

	{
		err := newEvaluator().CheckInternalRedundancy(items)
		require.ErrorIs(t, err, domain.ErrPermissionAlreadyOwned, "expected ErrPermissionAlreadyOwned, got %v", err)
	}
}

func TestNormalizePermissionVariants(t *testing.T) {
	t.Parallel()

	// 三种入口对「空列表」的处理不同，迁移后必须保持各自语义。
	{
		got, err := domain.NormalizeNewPermissions(nil)
		require.NoError(t, err, "NormalizeNewPermissions(nil) = (%v, %v), want (nil, nil)", got, err)
		require.Nil(t, got, "NormalizeNewPermissions(nil) = (%v, %v), want (nil, nil)", got, err)
	}

	{
		got, err := domain.NormalizeReplacementPermissions(nil)
		require.NoError(t, err, "NormalizeReplacementPermissions(nil) = (%v, %v), want (empty slice, nil)", got, err)
		require.NotNil(t, got, "NormalizeReplacementPermissions(nil) = (%v, %v), want (empty slice, nil)", got, err)
		require.Empty(t, got, "NormalizeReplacementPermissions(nil) = (%v, %v), want (empty slice, nil)", got, err)
	}

	{
		_, err := domain.CollectPermissions(nil)
		require.ErrorIs(t, err, domain.ErrPermissionsRequired, "CollectPermissions(nil) error = %v, want ErrPermissionsRequired", err)
	}

	{
		_, err := domain.CollectPermissions([]domain.PermissionItem{{Resource: " ", Action: testGET}})
		require.ErrorIs(t, err, domain.ErrPermissionParamIncomplete, "CollectPermissions(blank resource) error = %v, want ErrPermissionParamIncomplete", err)
	}
}
