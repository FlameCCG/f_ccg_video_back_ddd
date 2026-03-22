package rbac

import (
	"context"

	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"
)

// Tree 读取全部菜单并组装成树结构。
func (s *MenuService) Tree(ctx context.Context) ([]*rbacdomain.Menu, error) {
	menus, err := s.menus.ListOrdered(ctx)
	if err != nil {
		return nil, rbacdomain.ErrGetFailed
	}

	return s.order.Tree(menus), nil
}

// RoleMenus 读取角色的直连菜单树与继承菜单树。
func (s *MenuService) RoleMenus(ctx context.Context, roleID uint) (RoleMenusResp, error) {
	role, err := s.roles.FindIDByID(ctx, roleID)
	if err != nil || role == nil {
		return RoleMenusResp{}, rbacdomain.ErrRoleNotFound
	}

	directMenus, err := s.menus.ListByRoleInherited(ctx, role.ID(), false)
	if err != nil {
		return RoleMenusResp{}, rbacdomain.ErrGetFailed
	}

	inheritMenus, err := s.menus.ListByRoleInherited(ctx, role.ID(), true)
	if err != nil {
		return RoleMenusResp{}, rbacdomain.ErrGetFailed
	}

	return RoleMenusResp{
		DirectMenus:  ToMenuDTOs(s.order.Tree(directMenus)),
		InheritMenus: ToMenuDTOs(s.order.Tree(inheritMenus)),
	}, nil
}

// UserMenus 读取用户当前角色对应的菜单树；用户未设置当前角色时返回空列表。
func (s *MenuService) UserMenus(ctx context.Context, userID uint) ([]*rbacdomain.Menu, error) {
	roleID, found, err := s.userRoles.CurrentRoleID(ctx, userID)
	if err != nil || !found {
		return nil, rbacdomain.ErrUserNotFound
	}

	if roleID == 0 {
		return []*rbacdomain.Menu{}, nil
	}

	menus, err := s.menus.ListByRole(ctx, roleID)
	if err != nil {
		return nil, rbacdomain.ErrGetFailed
	}

	return s.order.Tree(menus), nil
}
