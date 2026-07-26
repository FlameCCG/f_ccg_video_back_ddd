package rbac

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"
	"f_ccg_video_back_ddd/pkg/constants"

	"go.uber.org/zap"
)

// InitService 默认 RBAC 初始化用例（原 service/rbac_service/init_service.go，供 CLI -rbac 使用）。
type InitService struct {
	roles     domain.RoleRepository
	menus     domain.MenuRepository
	roleMenus domain.RoleMenuRepository
	enforcer  Enforcer
}

// NewInitService 构造默认 RBAC 初始化用例。
func NewInitService(
	roles domain.RoleRepository,
	menus domain.MenuRepository,
	roleMenus domain.RoleMenuRepository,
	enforcer Enforcer,
) *InitService {
	return &InitService{roles: roles, menus: menus, roleMenus: roleMenus, enforcer: enforcer}
}

// InitDefaultRBAC 初始化默认角色、菜单和权限策略。
//
// 幂等：角色与菜单按名称/标题存在即跳过创建，仅补齐缺失字段与角色-菜单绑定；
// Casbin 分组与通配策略的写入沿用原实现（忽略单条写入结果，最后统一 SavePolicy）。
func (s *InitService) InitDefaultRBAC(ctx context.Context) error {
	if s.enforcer == nil {
		return domain.ErrCasbinNotInitialized
	}

	seedRoles := []*domain.Role{
		domain.NewRole(constants.AdminUserRole, "管理员，拥有所有权限"),
		domain.NewRole(constants.VipUserRole, "VIP用户，拥有部分特殊权限"),
		domain.NewRole(constants.CommonUserRole, "普通用户，拥有基本权限"),
	}

	for _, role := range seedRoles {
		// 与原实现一致：重名预检忽略 Count 的错误。
		count, _ := s.roles.CountByName(ctx, role.Name)
		if count == 0 {
			if err := s.roles.Create(ctx, role); err != nil {
				zap.L().Error("create role failed", zap.Error(err))

				return err
			}
		}
	}

	adminRole, _ := s.roles.FindByName(ctx, constants.AdminUserRole)
	vipRole, _ := s.roles.FindByName(ctx, constants.VipUserRole)
	userRole, _ := s.roles.FindByName(ctx, constants.CommonUserRole)

	if adminRole == nil || vipRole == nil || userRole == nil ||
		adminRole.ID == 0 || vipRole.ID == 0 || userRole.ID == 0 {
		return domain.ErrRoleInitFailed
	}

	if err := s.seedMenus(ctx, adminRole.ID, vipRole.ID, userRole.ID); err != nil {
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

	// parentID 解析父菜单主键；无父菜单时返回 0（写入 parent_id = 0，与原实现一致）。
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
			return nil, domain.ErrParentMenuNotFound
		}

		menuIDByTitle[title] = parent.ID
		pid := parent.ID

		return &pid, nil
	}

	for _, seed := range buildMenuSeeds() {
		pid, err := parentID(seed.ParentTitle)
		if err != nil {
			zap.L().Error("query parent menu failed", zap.Error(err),
				zap.String("title", seed.Title), zap.String("parent", seed.ParentTitle))

			return err
		}

		menuModel, err := s.menus.FindByTitle(ctx, seed.Title)
		if err != nil {
			zap.L().Error("query menu failed", zap.Error(err))

			return err
		}

		if menuModel == nil {
			menu := domain.NewMenu(
				seed.Title, seed.TitleEn, seed.TitleJa, seed.Icon,
				seed.Path, seed.Name, seed.Component, seed.KeepAlive, pid, seed.SortOrder,
			)
			if err := s.menus.Create(ctx, menu); err != nil {
				zap.L().Error("create menu failed", zap.Error(err))

				return err
			}

			menuModel = menu
		}

		update := seedMenuUpdate(seed, menuModel, pid)
		if !update.IsEmpty() {
			if err := s.menus.UpdateFields(ctx, menuModel.ID, update); err != nil {
				zap.L().Error("update menu failed", zap.Error(err))

				return err
			}
		}

		menuIDByTitle[seed.Title] = menuModel.ID

		if err := s.roleMenus.EnsureRoleMenu(ctx, adminRoleID, menuModel.ID); err != nil {
			zap.L().Error("bind admin menu failed", zap.Error(err))

			return err
		}

		if _, ok := commonRoleMenuTitles[menuModel.Title]; ok {
			if err := s.roleMenus.EnsureRoleMenu(ctx, userRoleID, menuModel.ID); err != nil {
				zap.L().Error("bind user menu failed", zap.Error(err))

				return err
			}
		}

		if _, ok := vipRoleMenuTitles[menuModel.Title]; ok {
			if err := s.roleMenus.EnsureRoleMenu(ctx, vipRoleID, menuModel.ID); err != nil {
				zap.L().Error("bind vip menu failed", zap.Error(err))

				return err
			}
		}
	}

	return nil
}

// seedMenuUpdate 计算既有菜单相对种子定义需要补齐的字段（只补空、不覆盖已有值）。
func seedMenuUpdate(seed menuSeed, current *domain.Menu, pid *uint) domain.MenuFieldUpdate {
	var update domain.MenuFieldUpdate

	if seed.TitleEn != "" && current.TitleEn != seed.TitleEn {
		titleEn := seed.TitleEn
		update.TitleEn = &titleEn
	}

	if seed.TitleJa != "" && current.TitleJa != seed.TitleJa {
		titleJa := seed.TitleJa
		update.TitleJa = &titleJa
	}

	if current.Icon == "" && seed.Icon != "" {
		icon := seed.Icon
		update.Icon = &icon
	}

	if current.Path == "" && seed.Path != "" {
		path := seed.Path
		update.Path = &path
	}

	if current.Name == "" && seed.Name != "" {
		name := seed.Name
		update.Name = &name
	}

	if current.Component == "" && seed.Component != "" {
		component := seed.Component
		update.Component = &component
	}

	if !current.KeepAlive && seed.KeepAlive {
		keepAlive := seed.KeepAlive
		update.KeepAlive = &keepAlive
	}

	if current.SortOrder == 0 && seed.SortOrder != 0 {
		sortOrder := seed.SortOrder
		update.SortOrder = &sortOrder
	}

	if seed.ParentTitle != "" {
		if current.ParentID == nil || *current.ParentID == 0 {
			value := *pid
			update.ParentID = &value
		}
	} else if current.ParentID == nil {
		zero := uint(0)
		update.ParentID = &zero
	}

	return update
}
