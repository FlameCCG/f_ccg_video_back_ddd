package entity_test

import (
	"errors"
	"testing"

	rbacerrors "f_ccg_video_back_ddd/internal/domain/rbac/domainerr"
	"f_ccg_video_back_ddd/internal/domain/rbac/entity"
	rbacvo "f_ccg_video_back_ddd/internal/domain/rbac/valueobject"
)

func TestMenuRejectsUnsafeChangeWithoutPartialMutation(t *testing.T) {
	menu := &entity.Menu{Title: "原菜单", Icon: "old"}
	title, icon := "新菜单", "<svg onload=\"alert(1)\"></svg>"
	if _, err := menu.Revise(entity.MenuFieldUpdate{Title: &title, Icon: &icon}); !errors.Is(err, rbacvo.ErrMenuIconUnsafe) || menu.Title != "原菜单" || menu.Icon != "old" {
		t.Fatalf("invalid revision changed menu: menu=%+v err=%v", menu, err)
	}
	if err := menu.EnsureDeletable(1); !errors.Is(err, rbacerrors.ErrDeleteChildMenusFirst) {
		t.Fatalf("menu with children must not be deletable: %v", err)
	}
}

func TestRoleRevisionRejectsEmptyNameAndNormalizesValidChange(t *testing.T) {
	role := &entity.Role{Name: "old", Desc: "original"}
	name, desc := "  ", " changed "
	if _, err := role.Revise(&name, &desc); !errors.Is(err, rbacerrors.ErrRoleNameEmpty) || role.Desc != "original" {
		t.Fatalf("invalid name must not partially apply description: role=%+v err=%v", role, err)
	}
	name = " new "
	change, err := role.Revise(&name, &desc)
	if err != nil || role.Name != "new" || role.Desc != "changed" || *change.Name != role.Name {
		t.Fatalf("valid revision: role=%+v change=%+v err=%v", role, change, err)
	}
	if err := role.EnsureDeletable(1); !errors.Is(err, rbacerrors.ErrRoleHasUsers) {
		t.Fatalf("assigned role must not be deletable: %v", err)
	}
}
