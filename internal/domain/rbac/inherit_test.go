package rbac_test

import (
	"strings"
	"testing"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"
)

func TestFormatAlreadyInheritedMsg(t *testing.T) {
	msg := domain.FormatAlreadyInheritedMsg("普通用户", domain.InheritKindIndirect, "大会员")
	if msg == "" {
		t.Fatal("expected non-empty msg")
	}

	if !strings.Contains(msg, "普通用户") || !strings.Contains(msg, "大会员") {
		t.Fatalf("unexpected msg: %s", msg)
	}

	directMsg := domain.FormatAlreadyInheritedMsg("大会员", domain.InheritKindDirect, "大会员")
	if !strings.Contains(directMsg, "直接") {
		t.Fatalf("expected direct hint, got: %s", directMsg)
	}
}

func TestFormatIndirectUnlinkMsg(t *testing.T) {
	msg := domain.FormatIndirectUnlinkMsg("普通用户", "大会员")
	if !strings.Contains(msg, "间接") || !strings.Contains(msg, "大会员") {
		t.Fatalf("unexpected msg: %s", msg)
	}
}

func TestContainsRoleName(t *testing.T) {
	if !domain.ContainsRoleName([]string{"大会员", "普通用户"}, "大会员") {
		t.Fatal("expected hit")
	}

	if domain.ContainsRoleName([]string{"大会员"}, "超级管理员") {
		t.Fatal("expected miss")
	}
}

func TestBuildMenuTree(t *testing.T) {
	// 空输入返回 nil（响应体表现为 data:null），与原 pkg/recursion 行为一致。
	if got := domain.BuildMenuTree(nil); got != nil {
		t.Fatalf("BuildMenuTree(nil) = %v, want nil", got)
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
	if len(tree) != 3 {
		t.Fatalf("roots = %d, want 3", len(tree))
	}

	if len(tree[0].Children) != 1 || tree[0].Children[0].ID != 2 {
		t.Fatalf("unexpected children of root 1: %+v", tree[0].Children)
	}

	if tree[1].ID != 3 || tree[2].ID != 4 {
		t.Fatalf("unexpected fallback roots: %d, %d", tree[1].ID, tree[2].ID)
	}
}

func TestBuildRoleInheritTree(t *testing.T) {
	roles := []*domain.Role{
		{ID: 1, Name: "超级管理员", Desc: "admin"},
		{ID: 2, Name: "大会员"},
		{ID: 3, Name: "普通用户"},
	}
	// g 策略语义：child 继承 parent，因此树的父子方向与策略相反。
	policies := [][]string{
		{"超级管理员", "大会员"},
		{"大会员", "普通用户"},
		{"user:1", "超级管理员"},
		{"未知角色", "普通用户"},
		{"残缺行"},
	}

	tree := domain.BuildRoleInheritTree(roles, policies)
	if len(tree) != 1 || tree[0].Name != "普通用户" {
		t.Fatalf("unexpected roots: %+v", tree)
	}

	if len(tree[0].Children) != 1 || tree[0].Children[0].Name != "大会员" {
		t.Fatalf("unexpected level 2: %+v", tree[0].Children)
	}

	if len(tree[0].Children[0].Children) != 1 || tree[0].Children[0].Children[0].Name != "超级管理员" {
		t.Fatalf("unexpected level 3: %+v", tree[0].Children[0].Children)
	}

	filtered := domain.FilterRoleInheritTree(tree, "大会员")
	if len(filtered) != 1 || filtered[0].Name != "普通用户" {
		t.Fatalf("unexpected filtered roots: %+v", filtered)
	}

	if len(filtered[0].Children) != 1 || filtered[0].Children[0].Name != "大会员" {
		t.Fatalf("unexpected filtered subtree: %+v", filtered[0].Children)
	}

	if got := domain.FilterRoleInheritTree(tree, "不存在的角色"); len(got) != 0 {
		t.Fatalf("expected empty result, got %+v", got)
	}
}

func TestBuildRoleInheritTreeHandlesCycle(t *testing.T) {
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
	if len(tree) == 0 {
		t.Fatal("expected cyclic roles to still be emitted")
	}
}
