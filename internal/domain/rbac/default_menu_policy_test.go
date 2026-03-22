package rbac_test

import (
	"testing"

	rbac "f_ccg_video_back_ddd/internal/domain/rbac"
)

func TestDefaultMenuEligibilityPreservesHistoricalRoleAllowlist(t *testing.T) {
	policy := rbac.DefaultMenuPolicy{}
	for _, title := range []string{"运营总览", "数据看板"} {
		if !policy.CommonRoleAllowed(title) || !policy.VIPRoleAllowed(title) {
			t.Fatalf("shared default menu denied: %s", title)
		}
	}
	for _, title := range []string{
		"内容管理", "社区治理", "营销推广", "用户管理", "系统权限", "稿件审核", "视频列表",
		"视频回收站", "分区配置", "举报处理", "评论管控", "弹幕管控", "动态管理",
		"轮播图管理", "全站通知", "封禁管理", "封禁记录", "角色管理", "菜单权限", "接口资源",
	} {
		if policy.CommonRoleAllowed(title) || !policy.VIPRoleAllowed(title) {
			t.Fatalf("VIP-only default eligibility changed: %s", title)
		}
	}
	// 展示名称与授权名称分别处理，默认授权范围固定，不额外进行空白归一化。
	for _, title := range []string{"", "仪表盘", "系统配置", "用户列表", "运营总览 ", "新菜单"} {
		if policy.CommonRoleAllowed(title) || policy.VIPRoleAllowed(title) {
			t.Fatalf("unexpected default menu grant: %q", title)
		}
	}
}
