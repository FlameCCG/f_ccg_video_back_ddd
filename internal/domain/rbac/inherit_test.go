package rbac_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"
)

const (
	testVIPRole        = "大会员"
	testSuperAdminRole = "超级管理员"
)

func TestFormatAlreadyInheritedMsg(t *testing.T) {
	t.Parallel()

	msg := domain.FormatAlreadyInheritedMsg(testRegularRole, domain.InheritKindIndirect, testVIPRole)
	require.NotEmpty(t, msg, "expected non-empty msg")

	require.Contains(t, msg, testRegularRole, "unexpected msg: %s", msg)
	require.Contains(t, msg, testVIPRole, "unexpected msg: %s", msg)

	directMsg := domain.FormatAlreadyInheritedMsg(testVIPRole, domain.InheritKindDirect, testVIPRole)
	require.Contains(t, directMsg, "直接", "expected direct hint, got: %s", directMsg)
}

func TestFormatIndirectUnlinkMsg(t *testing.T) {
	t.Parallel()

	msg := domain.FormatIndirectUnlinkMsg(testRegularRole, testVIPRole)
	require.Contains(t, msg, "间接", "unexpected msg: %s", msg)
	require.Contains(t, msg, testVIPRole, "unexpected msg: %s", msg)
}

func TestContainsRoleName(t *testing.T) {
	t.Parallel()

	require.True(t, domain.ContainsRoleName([]string{testVIPRole, testRegularRole}, testVIPRole), "expected hit")

	require.False(t, domain.ContainsRoleName([]string{testVIPRole}, testSuperAdminRole), "expected miss")
}

func TestBuildMenuTree(t *testing.T) {
	t.Parallel()

	// 空输入返回 nil（响应体表现为 data:null），与原 pkg/recursion 行为一致。
	{
		got := domain.BuildMenuTree(nil)
		require.Nil(t, got, "BuildMenuTree(nil) = %v, want nil", got)
	}

	root := uint(0)
	parentID := uint(1)
	orphanParent := uint(99)

	menus := []*domain.Menu{
		{ID: 1, Title: "父", ParentID: &root},
		{ID: 2, Title: "子", ParentID: &parentID},
		{ID: 3, Title: "无父兜底", ParentID: &orphanParent},
		{ID: 4, Title: "顶级"},
	}

	tree := domain.BuildMenuTree(menus)
	require.Len(t, tree, 3, "roots = %d, want 3", len(tree))

	require.Len(t, tree[0].Children, 1, "unexpected children of root 1: %+v", tree[0].Children)
	require.EqualValues(t, 2, tree[0].Children[0].ID, "unexpected children of root 1: %+v", tree[0].Children)

	require.EqualValues(t, 3, tree[1].ID, "unexpected fallback roots: %d, %d", tree[1].ID, tree[2].ID)
	require.EqualValues(t, 4, tree[2].ID, "unexpected fallback roots: %d, %d", tree[1].ID, tree[2].ID)
}

func TestBuildRoleInheritTree(t *testing.T) {
	t.Parallel()

	roles := []*domain.Role{
		{ID: 1, Name: testSuperAdminRole, Desc: "admin"},
		{ID: 2, Name: testVIPRole},
		{ID: 3, Name: testRegularRole},
	}
	// g 策略语义：child 继承 parent，因此树的父子方向与策略相反。
	policies := [][]string{
		{testSuperAdminRole, testVIPRole},
		{testVIPRole, testRegularRole},
		{"user:1", testSuperAdminRole},
		{"未知角色", testRegularRole},
		{"残缺行"},
	}

	tree := domain.BuildRoleInheritTree(roles, policies)
	require.Len(t, tree, 1, "unexpected roots: %+v", tree)
	require.Equal(t, testRegularRole, tree[0].Name, "unexpected roots: %+v", tree)

	require.Len(t, tree[0].Children, 1, "unexpected level 2: %+v", tree[0].Children)
	require.Equal(t, testVIPRole, tree[0].Children[0].Name, "unexpected level 2: %+v", tree[0].Children)

	require.Len(t, tree[0].Children[0].Children, 1, "unexpected level 3: %+v", tree[0].Children[0].Children)
	require.Equal(t, testSuperAdminRole, tree[0].Children[0].Children[0].Name, "unexpected level 3: %+v", tree[0].Children[0].Children)

	filtered := domain.FilterRoleInheritTree(tree, testVIPRole)
	require.Len(t, filtered, 1, "unexpected filtered roots: %+v", filtered)
	require.Equal(t, testRegularRole, filtered[0].Name, "unexpected filtered roots: %+v", filtered)

	require.Len(t, filtered[0].Children, 1, "unexpected filtered subtree: %+v", filtered[0].Children)
	require.Equal(t, testVIPRole, filtered[0].Children[0].Name, "unexpected filtered subtree: %+v", filtered[0].Children)

	{
		got := domain.FilterRoleInheritTree(tree, "不存在的角色")
		require.Empty(t, got, "expected empty result, got %+v", got)
	}
}

func TestBuildRoleInheritTreeHandlesCycle(t *testing.T) {
	t.Parallel()

	roles := []*domain.Role{
		{ID: 1, Name: "A"},
		{ID: 2, Name: "B"},
	}
	// A 继承 B、B 继承 A：无根组件，需补齐输出且不得死循环。
	policies := [][]string{
		{"A", "B"},
		{"B", "A"},
	}

	tree := domain.BuildRoleInheritTree(roles, policies)
	require.NotEmpty(t, tree, "expected cyclic roles to still be emitted")
}

const testRegularRole = "普通用户"
