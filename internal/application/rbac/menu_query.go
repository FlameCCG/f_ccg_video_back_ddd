package rbac

import (
	"context"

	rbacerrors "f_ccg_video_back_ddd/internal/domain/rbac/domainerr"
	rbacentity "f_ccg_video_back_ddd/internal/domain/rbac/entity"
)

// Tree 读取全部菜单并组装成树结构。
func (s *MenuService) Tree(ctx context.Context) ([]*rbacentity.Menu, error) {
	menus, err := s.menus.ListOrdered(ctx)
	if err != nil {
		return nil, rbacerrors.ErrGetFailed
	}

	return rbacentity.BuildMenuTree(menus), nil
}

// RoleMenus 读取角色的直连菜单树与继承菜单树。
func (s *MenuService) RoleMenus(ctx context.Context, roleID uint) (RoleMenusResp, error) {
	role, err := s.roles.FindIDByID(ctx, roleID)
	if err != nil || role == nil {
		return RoleMenusResp{}, rbacerrors.ErrRoleNotFound
	}

	directMenus, err := s.menus.ListByRoleInherited(ctx, role.ID, false)
	if err != nil {
		return RoleMenusResp{}, rbacerrors.ErrGetFailed
	}

	inheritMenus, err := s.menus.ListByRoleInherited(ctx, role.ID, true)
	if err != nil {
		return RoleMenusResp{}, rbacerrors.ErrGetFailed
	}

	return RoleMenusResp{
		DirectMenus:  rbacentity.BuildMenuTree(directMenus),
		InheritMenus: rbacentity.BuildMenuTree(inheritMenus),
	}, nil
}

// UserMenus 读取用户当前角色对应的菜单树；用户未设置当前角色时返回空列表。
func (s *MenuService) UserMenus(ctx context.Context, userID uint) ([]*rbacentity.Menu, error) {
	roleID, found, err := s.userRoles.CurrentRoleID(ctx, userID)
	if err != nil || !found {
		return nil, rbacerrors.ErrUserNotFound
	}

	if roleID == 0 {
		return []*rbacentity.Menu{}, nil
	}

	menus, err := s.menus.ListByRole(ctx, roleID)
	if err != nil {
		return nil, rbacerrors.ErrGetFailed
	}

	return rbacentity.BuildMenuTree(menus), nil
}
