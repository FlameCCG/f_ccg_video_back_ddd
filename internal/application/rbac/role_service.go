package rbac

import (
	"context"
	"errors"

	rbacerrors "f_ccg_video_back_ddd/internal/domain/rbac/domainerr"
	rbacentity "f_ccg_video_back_ddd/internal/domain/rbac/entity"
	rbacrepo "f_ccg_video_back_ddd/internal/domain/rbac/repository"
	rbacservice "f_ccg_video_back_ddd/internal/domain/rbac/service"
	rbacvo "f_ccg_video_back_ddd/internal/domain/rbac/valueobject"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/pkg/casbins"

	"go.uber.org/zap"
)

// RoleService 角色用例服务。
type RoleService struct {
	txm       tx.Manager
	roles     rbacrepo.RoleRepository
	menus     rbacrepo.MenuRepository
	userRoles rbacrepo.UserRoleRepository
	enforcer  Enforcer
	logger    *zap.Logger
}

// NewRoleService 构造角色用例服务。
func NewRoleService(
	txm tx.Manager,
	roles rbacrepo.RoleRepository,
	menus rbacrepo.MenuRepository,
	userRoles rbacrepo.UserRoleRepository,
	enforcer Enforcer,
	logger *zap.Logger,
) *RoleService {
	return &RoleService{
		txm:       txm,
		roles:     roles,
		menus:     menus,
		userRoles: userRoles,
		enforcer:  enforcer,
		logger:    logger,
	}
}

// List 读取全部角色（含各自绑定的菜单）。
func (s *RoleService) List(ctx context.Context) ([]*rbacentity.Role, error) {
	roles, err := s.roles.ListWithMenus(ctx)
	if err != nil {
		return nil, rbacerrors.ErrGetFailed
	}

	return roles, nil
}

// Create 创建角色，在事务内写角色与菜单绑定，提交后写 Casbin 权限策略。
//
// 权限策略写入失败时角色已经创建，因此接口可能返回部分成功结果。
func (s *RoleService) Create(ctx context.Context, params CreateRoleParams) (result error) {
	scoped, finish, err := beginPolicyChange(s.enforcer)
	if err != nil {
		return err
	}
	defer finish(&result)
	service := *s
	service.enforcer = scoped
	s = &service
	// 重名预检不校验 Count 的错误，查询失败时按 0 处理并继续。
	count, _ := s.roles.CountByName(ctx, params.Name)
	if count > 0 {
		return rbacerrors.ErrRoleNameExists
	}

	var (
		role      = rbacentity.NewRole(params.Name, params.Desc)
		copyPerms []rbacvo.PermissionItem
		bizErr    error
	)

	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		bizErr = nil

		if err := s.roles.Create(txCtx, role); err != nil {
			s.logger.Error("创建角色失败", zap.Error(err))
			bizErr = rbacerrors.ErrCreateFailed

			return bizErr
		}

		var templateMenus []*rbacentity.Menu

		if params.CopyRoleID > 0 {
			copyRole, err := s.roles.FindWithMenusByID(txCtx, params.CopyRoleID)
			if err != nil || copyRole == nil {
				bizErr = rbacerrors.ErrTemplateRoleNotFound

				return bizErr
			}

			templateMenus = copyRole.Menus

			perms, err := s.enforcer.GetPermissionsForUser(copyRole.Name)
			if err != nil {
				bizErr = rbacerrors.ErrTemplatePermissionsRead

				return bizErr
			}

			copyPerms = append(copyPerms, rbacservice.NormalizeCasbinPolicyRows(perms)...)
		}

		ids := rbacentity.MergeMenuIDs(templateMenus, params.MenuIDs)
		if len(ids) == 0 {
			return nil
		}

		menus, err := s.menus.FindByIDs(txCtx, ids)
		if err != nil {
			bizErr = rbacerrors.ErrMenuNotFound

			return bizErr
		}

		if len(menus) != len(ids) {
			bizErr = rbacerrors.ErrPartialMenusNotFound

			return bizErr
		}

		if err := s.roles.AppendMenus(txCtx, role, menus); err != nil {
			bizErr = rbacerrors.ErrBindMenuFailed

			return bizErr
		}

		return nil
	}); err != nil {
		if bizErr != nil {
			return bizErr
		}

		s.logger.Error("提交事务失败", zap.Error(err))

		return rbacerrors.ErrCreateFailed
	}

	perms, err := rbacservice.NormalizeNewPermissions(params.Permissions)
	if err != nil {
		return err
	}

	perms = append(perms, copyPerms...)
	perms = rbacservice.DedupePermissions(perms)

	for _, p := range perms {
		if _, err := s.enforcer.AddPolicy(role.Name, p.Resource, p.Action); err != nil {
			return rbacerrors.ErrAddPermissionFailed
		}
	}

	if len(perms) > 0 {
		if err := s.enforcer.SavePolicy(); err != nil {
			return rbacerrors.ErrSavePermissionFailed
		}
	}

	return nil
}

// Update 更新角色名与描述；角色名变更时同步迁移 Casbin 策略与分组关系。
func (s *RoleService) Update(ctx context.Context, params UpdateRoleParams) (result error) {
	scoped, finish, err := beginPolicyChange(s.enforcer)
	if err != nil {
		return err
	}
	defer finish(&result)
	service := *s
	service.enforcer = scoped
	s = &service
	if params.Name == nil && params.Desc == nil {
		return rbacerrors.ErrNoUpdatableFields
	}

	role, err := s.roles.FindByID(ctx, params.RoleID)
	if err != nil || role == nil {
		return rbacerrors.ErrRoleNotFound
	}

	oldName := role.Name
	candidate := *role
	change, err := candidate.Revise(params.Name, params.Desc)
	if err != nil {
		return err
	}
	newName := candidate.Name
	nameUpdate, descUpdate := change.Name, change.Desc
	if nameUpdate != nil {
		// 重名预检忽略 Count 的错误。
		count, _ := s.roles.CountByName(ctx, newName)
		if count > 0 {
			return rbacerrors.ErrRoleNameExists
		}
	}

	if err := s.roles.UpdateNameAndDesc(ctx, role.ID, nameUpdate, descUpdate); err != nil {
		s.logger.Error("更新角色失败", zap.Error(err))

		return rbacerrors.ErrUpdateFailed
	}

	if newName != oldName {
		if err := s.renameInCasbin(oldName, newName); err != nil {
			s.logger.Error("同步 Casbin 角色名失败", zap.Error(err))
			if rollbackErr := s.roles.UpdateName(context.WithoutCancel(ctx), role.ID, oldName); rollbackErr != nil {
				s.logger.Error("RBAC role rename compensation failed", zap.Error(rollbackErr))
				return errors.Join(rbacerrors.ErrPermissionSyncFailed, err, rollbackErr)
			}

			return rbacerrors.ErrPermissionSyncFailed
		}
	}

	return nil
}

// renameInCasbin 把 oldName 的全部策略、子角色与父角色关系迁移到 newName。
func (s *RoleService) renameInCasbin(oldName, newName string) error {
	permissions, err := s.enforcer.GetPermissionsForUser(oldName)
	if err != nil {
		return err
	}

	users, err := s.enforcer.GetUsersForRole(oldName)
	if err != nil {
		return err
	}

	parents, err := s.enforcer.GetRolesForUser(oldName)
	if err != nil {
		return err
	}

	for _, user := range users {
		if _, err := s.enforcer.DeleteRoleForUser(user, oldName); err != nil {
			return err
		}

		if _, err := s.enforcer.AddRoleForUser(user, newName); err != nil {
			return err
		}
	}

	for _, parent := range parents {
		if _, err := s.enforcer.DeleteRoleForUser(oldName, parent); err != nil {
			return err
		}

		if _, err := s.enforcer.AddRoleForUser(newName, parent); err != nil {
			return err
		}
	}

	if len(permissions) > 0 {
		for _, p := range permissions {
			if len(p) < 3 {
				return rbacerrors.ErrInvalidCasbinPermission
			}

			if _, err := s.enforcer.AddPolicy(newName, p[1], p[2]); err != nil {
				return err
			}
		}

		if _, err := s.enforcer.RemoveFilteredPolicy(0, oldName); err != nil {
			return err
		}
	}

	return s.enforcer.SavePolicy()
}

// Delete 删除角色：角色下仍有用户时拒绝；删除后清理 Casbin 策略。
func (s *RoleService) Delete(ctx context.Context, roleID uint) (result error) {
	scoped, finish, err := beginPolicyChange(s.enforcer)
	if err != nil {
		return err
	}
	defer finish(&result)
	service := *s
	service.enforcer = scoped
	s = &service
	role, err := s.roles.FindByID(ctx, roleID)
	if err != nil || role == nil {
		return rbacerrors.ErrRoleNotFound
	}

	userCount, err := s.roles.UserCount(ctx, role.ID)
	if err != nil {
		// 预加载用户失败与角色不存在使用同一对外文案。
		return rbacerrors.ErrRoleNotFound
	}

	// 检查是否有用户使用该角色
	if err := role.EnsureDeletable(userCount); err != nil {
		return err
	}

	// 删除角色（持久化钩子会自动清理关联）

	// 删除 Casbin 中该角色的所有权限
	if _, err := s.enforcer.DeleteRole(role.Name); err != nil {
		s.logger.Error("删除 Casbin 角色失败", zap.Error(err))

		return rbacerrors.ErrRolePolicyCleanupFailed
	}

	if err := s.enforcer.SavePolicy(); err != nil {
		s.logger.Error("保存 Casbin 策略失败", zap.Error(err))

		return rbacerrors.ErrRolePolicySaveFailed
	}

	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		if err := s.roles.DeleteBindings(txCtx, role.ID); err != nil {
			return err
		}
		return s.roles.DeleteByID(txCtx, role.ID)
	}); err != nil {
		return errors.Join(rbacerrors.ErrDeleteFailed, err)
	}

	return nil
}

// UserRoles 读取用户已绑定的角色列表。
func (s *RoleService) UserRoles(ctx context.Context, userID uint) ([]*rbacentity.Role, error) {
	roles, err := s.userRoles.RolesByUser(ctx, userID)
	if err != nil {
		return nil, rbacerrors.ErrUserNotFound
	}

	return roles, nil
}

// UserPermissions 读取用户最终权限列表（经其角色隐式展开）。
func (s *RoleService) UserPermissions(ctx context.Context, userID uint) ([]UserPermissionItem, error) {
	exists, err := s.userRoles.UserExists(ctx, userID)
	if err != nil || !exists {
		return nil, rbacerrors.ErrUserNotFound
	}

	sub := casbins.GetUserSub(userID)

	perms, err := s.enforcer.GetImplicitPermissionsForUser(sub)
	if err != nil {
		return nil, rbacerrors.ErrQueryFailed
	}

	items := make([]UserPermissionItem, 0, len(perms))

	for _, p := range perms {
		if len(p) < 3 {
			continue
		}

		items = append(items, UserPermissionItem{
			Resource: p[1],
			Action:   p[2],
		})
	}

	return items, nil
}

// AssignUserRoles 为用户追加角色绑定，并同步 Casbin 分组策略。
func (s *RoleService) AssignUserRoles(ctx context.Context, userID uint, roleIDs []uint) error {
	return s.changeUserRoles(ctx, userID, roleIDs, "assign")
}

// RemoveUserRoles removes only the requested direct roles after database commit.
func (s *RoleService) RemoveUserRoles(ctx context.Context, userID uint, roleIDs []uint) error {
	return s.changeUserRoles(ctx, userID, roleIDs, "remove")
}

// ReplaceUserRoles replaces the direct role set after database commit.
func (s *RoleService) ReplaceUserRoles(ctx context.Context, userID uint, roleIDs []uint) error {
	return s.changeUserRoles(ctx, userID, roleIDs, "replace")
}
func (s *RoleService) changeUserRoles(ctx context.Context, userID uint, roleIDs []uint, mode string) (result error) {
	scoped, finish, err := beginPolicyChange(s.enforcer)
	if err != nil {
		return err
	}
	defer finish(&result)
	service := *s
	service.enforcer = scoped
	s = &service
	var oldRoles, newRoles []*rbacentity.Role
	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		var err error
		oldRoles, err = s.userRoles.RolesByUser(txCtx, userID)
		if err != nil {
			return s.mapUserLookupError(err)
		}
		selected, err := s.roles.FindByIDs(txCtx, roleIDs)
		if err != nil {
			return rbacerrors.ErrQueryRoleFailed
		}
		if len(selected) != len(roleIDs) {
			return rbacerrors.ErrSomeRolesNotFound
		}
		switch mode {
		case "assign":
			newRoles = append(newRoles, oldRoles...)
			for _, r := range selected {
				for _, old := range oldRoles {
					if r.ID == old.ID {
						return rbacerrors.DuplicateRoleAssignedError(r.Name)
					}
				}
				newRoles = append(newRoles, r)
			}
		case "remove":
			remove := map[uint]bool{}
			for _, r := range selected {
				remove[r.ID] = true
			}
			for _, r := range oldRoles {
				if !remove[r.ID] {
					newRoles = append(newRoles, r)
				}
			}
		default:
			newRoles = selected
		}
		return s.userRoles.ReplaceRoles(txCtx, userID, newRoles)
	}); err != nil {
		return err
	}
	// No Casbin write is made before the user_roles transaction commits.
	syncErr := func() error {
		sub := casbins.GetUserSub(userID)
		existing, err := s.enforcer.GetRolesForUser(sub)
		if err != nil {
			return err
		}
		for _, name := range existing {
			if _, err := s.enforcer.DeleteRoleForUser(sub, name); err != nil {
				return err
			}
		}
		for _, r := range newRoles {
			if _, err := s.enforcer.AddRoleForUser(sub, r.Name); err != nil {
				return err
			}
		}
		return s.enforcer.SavePolicy()
	}()
	if syncErr != nil {
		rollbackErr := s.txm.Do(context.WithoutCancel(ctx), func(txCtx context.Context) error { return s.userRoles.ReplaceRoles(txCtx, userID, oldRoles) })
		if rollbackErr != nil {
			s.logger.Error("RBAC role binding compensation failed", zap.Error(rollbackErr))
		}
		return errors.Join(rbacerrors.ErrPermissionSyncFailed, syncErr, rollbackErr)
	}
	return nil
}

// mapUserLookupError 将用户查询错误归一为接口文案。
func (s *RoleService) mapUserLookupError(err error) error {
	if errors.Is(err, rbacerrors.ErrUserNotFound) {
		return rbacerrors.ErrUserNotFound
	}

	s.logger.Error("查询用户失败", zap.Error(err))

	return rbacerrors.ErrQueryUserFailed
}
