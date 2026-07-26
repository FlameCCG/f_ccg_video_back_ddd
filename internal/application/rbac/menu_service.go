package rbac

import (
	"context"
	"strings"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"

	"go.uber.org/zap"
)

// MenuService 菜单用例服务（原 api/rbac_api 中 menu/role-menu 相关处理器的业务部分）。
type MenuService struct {
	roles     domain.RoleRepository
	menus     domain.MenuRepository
	roleMenus domain.RoleMenuRepository
	userRoles domain.UserRoleRepository
}

// NewMenuService 构造菜单用例服务。
func NewMenuService(
	roles domain.RoleRepository,
	menus domain.MenuRepository,
	roleMenus domain.RoleMenuRepository,
	userRoles domain.UserRoleRepository,
) *MenuService {
	return &MenuService{roles: roles, menus: menus, roleMenus: roleMenus, userRoles: userRoles}
}

// Tree 读取全部菜单并组装成树结构。
func (s *MenuService) Tree(ctx context.Context) ([]*domain.Menu, error) {
	menus, err := s.menus.ListOrdered(ctx)
	if err != nil {
		return nil, domain.ErrGetFailed
	}

	// 组装成树结构
	return domain.BuildMenuTree(menus), nil
}

// Create 创建菜单。
func (s *MenuService) Create(ctx context.Context, params CreateMenuParams) error {
	titleEn, ok := domain.TrimOptional(params.TitleEn)
	if !ok {
		return domain.ErrMenuTitleEnEmpty
	}

	titleJa, ok := domain.TrimOptional(params.TitleJa)
	if !ok {
		return domain.ErrMenuTitleJaEmpty
	}

	path, ok := domain.TrimOptional(params.Path)
	if !ok {
		return domain.ErrMenuPathEmpty
	}

	name, ok := domain.TrimOptional(params.Name)
	if !ok {
		return domain.ErrMenuNameEmpty
	}

	component, ok := domain.TrimOptional(params.Component)
	if !ok {
		return domain.ErrMenuComponentEmpty
	}

	menu := domain.NewMenu(
		params.Title, titleEn, titleJa, params.Icon,
		path, name, component, params.KeepAlive, params.ParentID, params.SortOrder,
	)

	// 查看是否有父菜单
	if params.ParentID != nil {
		exists, err := s.menus.ExistsByID(ctx, *params.ParentID)
		if err != nil || !exists {
			return domain.ErrParentMenuNotFound
		}
	}

	// 查看是否已经存在同名菜单
	existing, err := s.menus.FindByTitle(ctx, params.Title)
	if err == nil && existing != nil {
		return domain.ErrMenuExists
	}

	// 创建菜单
	if err := s.menus.Create(ctx, menu); err != nil {
		return domain.ErrCreateFailed
	}

	return nil
}

// Update 按需更新菜单字段。
func (s *MenuService) Update(ctx context.Context, params UpdateMenuParams) error {
	if params.Title == nil && params.TitleEn == nil && params.TitleJa == nil && params.Icon == nil &&
		params.Path == nil && params.Name == nil && params.Component == nil && params.KeepAlive == nil &&
		params.ParentID == nil && params.SortOrder == nil {
		return domain.ErrNoUpdatableFields
	}

	menu, err := s.menus.FindByID(ctx, params.ID)
	if err != nil || menu == nil {
		return domain.ErrMenuNotFound
	}

	var update domain.MenuFieldUpdate

	if params.Title != nil {
		title := strings.TrimSpace(*params.Title)
		if title == "" {
			return domain.ErrMenuTitleEmpty
		}

		if title != menu.Title {
			// 与原实现一致：重名预检忽略 Count 的错误。
			count, _ := s.menus.CountByTitleExcluding(ctx, title, menu.ID)
			if count > 0 {
				return domain.ErrMenuExists
			}

			update.Title = &title
		}
	}

	if params.TitleEn != nil {
		titleEn := strings.TrimSpace(*params.TitleEn)
		if titleEn == "" {
			return domain.ErrMenuTitleEnEmpty
		}

		if titleEn != menu.TitleEn {
			update.TitleEn = &titleEn
		}
	}

	if params.TitleJa != nil {
		titleJa := strings.TrimSpace(*params.TitleJa)
		if titleJa == "" {
			return domain.ErrMenuTitleJaEmpty
		}

		if titleJa != menu.TitleJa {
			update.TitleJa = &titleJa
		}
	}

	if params.Icon != nil {
		icon := strings.TrimSpace(*params.Icon)
		update.Icon = &icon
	}

	if params.Path != nil {
		path := strings.TrimSpace(*params.Path)
		if path == "" {
			return domain.ErrMenuPathEmpty
		}

		update.Path = &path
	}

	if params.Name != nil {
		name := strings.TrimSpace(*params.Name)
		if name == "" {
			return domain.ErrMenuNameEmpty
		}

		update.Name = &name
	}

	if params.Component != nil {
		component := strings.TrimSpace(*params.Component)
		if component == "" {
			return domain.ErrMenuComponentEmpty
		}

		update.Component = &component
	}

	if params.KeepAlive != nil {
		update.KeepAlive = params.KeepAlive
	}

	if params.ParentID != nil {
		if *params.ParentID == menu.ID {
			return domain.ErrParentMenuIsSelf
		}

		if *params.ParentID == 0 {
			update.ClearParent = true
		} else {
			exists, err := s.menus.ExistsByID(ctx, *params.ParentID)
			if err != nil || !exists {
				return domain.ErrParentMenuNotFound
			}

			update.ParentID = params.ParentID
		}
	}

	if params.SortOrder != nil {
		update.SortOrder = params.SortOrder
	}

	if update.IsEmpty() {
		return domain.ErrNoUpdatableFields
	}

	if err := s.menus.UpdateFields(ctx, menu.ID, update); err != nil {
		return domain.ErrUpdateFailed
	}

	return nil
}

// Delete 删除菜单：存在子菜单时拒绝，删除前先清理角色-菜单关联。
func (s *MenuService) Delete(ctx context.Context, menuID uint) error {
	menu, err := s.menus.FindByID(ctx, menuID)
	if err != nil || menu == nil {
		return domain.ErrMenuNotFound
	}

	childCount, err := s.menus.CountChildren(ctx, menu.ID)
	if err != nil {
		return domain.ErrQueryFailed
	}

	if childCount > 0 {
		return domain.ErrDeleteChildMenusFirst
	}

	if err := s.roleMenus.DeleteByMenu(ctx, menu.ID); err != nil {
		return domain.ErrDeleteFailed
	}

	if err := s.menus.DeleteByID(ctx, menu.ID); err != nil {
		return domain.ErrDeleteFailed
	}

	return nil
}

// AssignRoleMenus 用给定菜单集合整体替换角色的直连菜单（继承菜单不受影响）。
func (s *MenuService) AssignRoleMenus(ctx context.Context, roleID uint, menuIDs []uint) error {
	role, err := s.roles.FindIDByID(ctx, roleID)
	if err != nil || role == nil {
		return domain.ErrRoleNotFound
	}

	inheritMenuIDs, err := s.roleMenus.InheritedMenuIDs(ctx, role.ID)
	if err != nil {
		return domain.ErrQueryFailed
	}

	filteredIDs := domain.DedupeMenuIDs(menuIDs, inheritMenuIDs)

	if len(filteredIDs) == 0 {
		if err := s.roleMenus.DeleteDirectByRole(ctx, role.ID); err != nil {
			zap.L().Error("清空菜单失败", zap.Error(err))

			return domain.ErrUpdateFailed
		}

		return nil
	}

	menus, err := s.menus.FindByIDs(ctx, filteredIDs)
	if err != nil {
		zap.L().Error("查询菜单失败", zap.Error(err))

		return domain.ErrQueryMenuFailed
	}

	// 验证菜单是否都存在
	if len(menus) != len(filteredIDs) {
		return domain.ErrSomeMenusNotFound
	}

	if err := s.roleMenus.DeleteDirectByRoleExcept(ctx, role.ID, filteredIDs); err != nil {
		zap.L().Error("更新菜单失败", zap.Error(err))

		return domain.ErrUpdateFailed
	}

	if err := s.roleMenus.CreateIgnoreConflict(ctx, domain.NewDirectRoleMenus(role.ID, menus)); err != nil {
		zap.L().Error("更新菜单失败", zap.Error(err))

		return domain.ErrUpdateFailed
	}

	return nil
}

// RemoveRoleMenus 移除角色的指定直连菜单（继承菜单不可移除）。
func (s *MenuService) RemoveRoleMenus(ctx context.Context, roleID uint, menuIDs []uint) error {
	role, err := s.roles.FindIDByID(ctx, roleID)
	if err != nil || role == nil {
		return domain.ErrRoleNotFound
	}

	inheritMenuIDs, err := s.roleMenus.InheritedMenuIDs(ctx, role.ID)
	if err != nil {
		return domain.ErrQueryFailed
	}

	filteredIDs := domain.DedupeMenuIDs(menuIDs, inheritMenuIDs)
	if len(filteredIDs) == 0 {
		return nil
	}

	// 批量查询要移除的菜单
	menus, err := s.menus.FindByIDs(ctx, filteredIDs)
	if err != nil {
		return domain.ErrQueryMenuFailed
	}

	// 验证菜单是否都存在
	if len(menus) != len(filteredIDs) {
		return domain.ErrSomeMenusNotFound
	}

	// 批量移除直连菜单
	if err := s.roleMenus.DeleteDirectByRoleAndMenus(ctx, role.ID, filteredIDs); err != nil {
		zap.L().Error("移除菜单失败", zap.Error(err))

		return domain.ErrRemoveFailed
	}

	return nil
}

// RoleMenus 读取角色的直连菜单树与继承菜单树。
func (s *MenuService) RoleMenus(ctx context.Context, roleID uint) (RoleMenusResp, error) {
	role, err := s.roles.FindIDByID(ctx, roleID)
	if err != nil || role == nil {
		return RoleMenusResp{}, domain.ErrRoleNotFound
	}

	directMenus, err := s.menus.ListByRoleInherited(ctx, role.ID, false)
	if err != nil {
		return RoleMenusResp{}, domain.ErrGetFailed
	}

	inheritMenus, err := s.menus.ListByRoleInherited(ctx, role.ID, true)
	if err != nil {
		return RoleMenusResp{}, domain.ErrGetFailed
	}

	return RoleMenusResp{
		DirectMenus:  domain.BuildMenuTree(directMenus),
		InheritMenus: domain.BuildMenuTree(inheritMenus),
	}, nil
}

// UserMenus 读取用户当前角色对应的菜单树；用户未设置当前角色时返回空列表。
func (s *MenuService) UserMenus(ctx context.Context, userID uint) ([]*domain.Menu, error) {
	roleID, found, err := s.userRoles.CurrentRoleID(ctx, userID)
	if err != nil || !found {
		return nil, domain.ErrUserNotFound
	}

	if roleID == 0 {
		return []*domain.Menu{}, nil
	}

	menus, err := s.menus.ListByRole(ctx, roleID)
	if err != nil {
		return nil, domain.ErrGetFailed
	}

	// 构建菜单树
	return domain.BuildMenuTree(menus), nil
}
