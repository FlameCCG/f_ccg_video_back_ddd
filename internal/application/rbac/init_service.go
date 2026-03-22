package rbac

import (
	"context"

	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"
	"f_ccg_video_back_ddd/pkg/constants"

	"go.uber.org/zap"
)

// InitService 默认 RBAC 初始化用例（供 CLI -rbac 使用）。
type InitService struct {
	roles     rbacdomain.RoleRepository
	menus     rbacdomain.MenuRepository
	roleMenus rbacdomain.RoleMenuRepository
	enforcer  Enforcer
	logger    *zap.Logger
}

// NewInitService 构造默认 RBAC 初始化用例。
func NewInitService(
	roles rbacdomain.RoleRepository,
	menus rbacdomain.MenuRepository,
	roleMenus rbacdomain.RoleMenuRepository,
	enforcer Enforcer,
	logger *zap.Logger,
) *InitService {
	return &InitService{roles: roles, menus: menus, roleMenus: roleMenus, enforcer: enforcer, logger: logger}
}

// InitDefaultRBAC 初始化默认角色、菜单与权限策略。
//
// Casbin 分组和通配策略逐条写入，忽略单条结果，最后统一 SavePolicy。
func (s *InitService) InitDefaultRBAC(ctx context.Context) error {
	if s.enforcer == nil {
		return rbacdomain.ErrCasbinNotInitialized
	}

	seedRoles := []*rbacdomain.Role{rbacdomain.NewRole(constants.AdminUserRole, "管理员，拥有所有权限"), rbacdomain.NewRole(constants.VipUserRole, "VIP用户，拥有部分特殊权限"), rbacdomain.NewRole(constants.CommonUserRole, "普通用户，拥有基本权限")}

	for _, role := range seedRoles {
		// 重名预检忽略 Count 的错误。
		count, _ := s.roles.CountByName(ctx, role.Name())
		if count == 0 {
			if _, err := s.roles.Create(ctx, role); err != nil {
				s.logger.Error("create role failed", zap.Error(err))

				return err
			}
		}
	}

	adminRole, _ := s.roles.FindByName(ctx, constants.AdminUserRole)
	vipRole, _ := s.roles.FindByName(ctx, constants.VipUserRole)
	userRole, _ := s.roles.FindByName(ctx, constants.CommonUserRole)

	if adminRole == nil || vipRole == nil || userRole == nil ||
		adminRole.ID() == 0 || vipRole.ID() == 0 || userRole.ID() == 0 {
		return rbacdomain.ErrRoleInitFailed
	}

	if err := s.seedMenus(ctx, adminRole.ID(), vipRole.ID(), userRole.ID()); err != nil {
		return err
	}

	_, _ = s.enforcer.AddRoleForUser(constants.VipUserRole, constants.CommonUserRole)
	_, _ = s.enforcer.AddRoleForUser(constants.AdminUserRole, constants.VipUserRole)

	defaultPolicies := [][]string{
		{constants.AdminUserRole, "/v1/admin/**", "*"},
		{constants.CommonUserRole, "/v1/common/**", "*"},
		{constants.VipUserRole, "/v1/vip/**", "*"},
	}

	for _, p := range defaultPolicies {
		_, _ = s.enforcer.AddPolicy(p[0], p[1], p[2])
	}

	return s.enforcer.SavePolicy()
}

// seedMenus 幂等地补齐默认菜单，并把菜单绑定到三个默认角色。
func (s *InitService) seedMenus(ctx context.Context, adminRoleID, vipRoleID, userRoleID uint) error {
	menuIDByTitle := make(map[string]uint)
	rootID := uint(0)

	// parentID 解析父菜单主键；无父菜单时返回 0。
	parentID := func(title string) (*uint, error) {
		if title == "" {
			return &rootID, nil
		}

		if id, ok := menuIDByTitle[title]; ok {
			pid := id

			return &pid, nil
		}

		parent, err := s.menus.FindByTitle(ctx, title)
		if err != nil {
			return nil, err
		}

		if parent == nil {
			return nil, rbacdomain.ErrParentMenuNotFound
		}

		menuIDByTitle[title] = parent.ID()
		pid := parent.ID()

		return &pid, nil
	}

	for _, seed := range buildMenuSeeds() {
		pid, err := parentID(seed.ParentTitle)
		if err != nil {
			s.logger.Error("query parent menu failed", zap.Error(err),
				zap.String("title", seed.Title), zap.String("parent", seed.ParentTitle))

			return err
		}

		menuModel, err := s.menus.FindByTitle(ctx, seed.Title)
		if err != nil {
			s.logger.Error("query menu failed", zap.Error(err))

			return err
		}

		if menuModel == nil {
			menu, err := rbacdomain.NewMenu(
				seed.Title, seed.TitleEn, seed.TitleJa, seed.Icon,
				seed.Path, seed.Name, seed.Component, seed.KeepAlive, pid, seed.SortOrder,
			)
			if err != nil {
				return err
			}
			menu, err = s.menus.Create(ctx, menu)
			if err != nil {
				s.logger.Error("create menu failed", zap.Error(err))

				return err
			}

			menuModel = menu
		}

		update := menuModel.SeedUpdate(rbacdomain.MenuDefaults{TitleEn: seed.TitleEn, TitleJa: seed.TitleJa, Icon: seed.Icon, Path: seed.Path, Name: seed.Name, Component: seed.Component, ParentTitle: seed.ParentTitle, KeepAlive: seed.KeepAlive, SortOrder: seed.SortOrder}, pid)
		if !update.IsEmpty() {
			if err := s.menus.UpdateFields(ctx, menuModel.ID(), update); err != nil {
				s.logger.Error("update menu failed", zap.Error(err))

				return err
			}
		}

		menuIDByTitle[seed.Title] = menuModel.ID()

		if err := s.roleMenus.EnsureRoleMenu(ctx, adminRoleID, menuModel.ID()); err != nil {
			s.logger.Error("bind admin menu failed", zap.Error(err))

			return err
		}

		if (rbacdomain.DefaultMenuPolicy{}).CommonRoleAllowed(menuModel.Title()) {
			if err := s.roleMenus.EnsureRoleMenu(ctx, userRoleID, menuModel.ID()); err != nil {
				s.logger.Error("bind user menu failed", zap.Error(err))

				return err
			}
		}

		if (rbacdomain.DefaultMenuPolicy{}).VIPRoleAllowed(menuModel.Title()) {
			if err := s.roleMenus.EnsureRoleMenu(ctx, vipRoleID, menuModel.ID()); err != nil {
				s.logger.Error("bind vip menu failed", zap.Error(err))

				return err
			}
		}
	}

	return nil
}
