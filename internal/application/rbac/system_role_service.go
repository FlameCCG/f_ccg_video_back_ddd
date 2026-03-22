package rbac

import (
	"context"
	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"
	"f_ccg_video_back_ddd/pkg/constants"
	"fmt"
	"go.uber.org/zap"
)

// SystemRoleService 处理一次性管理员角色初始化，复用调用方的事务 context。
type SystemRoleService struct {
	roles    rbacdomain.RoleRepository
	bindings rbacdomain.UserRoleRepository
	logger   *zap.Logger
}

func NewSystemRoleService(roles rbacdomain.RoleRepository, bindings rbacdomain.UserRoleRepository, logger *zap.Logger) *SystemRoleService {
	return &SystemRoleService{roles: roles, bindings: bindings, logger: logger}
}

func (s *SystemRoleService) EnsureAdminRole(ctx context.Context) (uint, string, bool, error) {
	role, err := s.roles.FindByName(ctx, constants.AdminUserRole)
	if err != nil {
		return 0, "", false, fmt.Errorf("查询管理员角色失败: %w", err)
	}
	if role != nil {
		return role.ID(), role.Name(), false, nil
	}
	s.logger.Warn("超级管理员角色不存在，正在自动创建...", zap.String("role", constants.AdminUserRole))
	role, err = s.roles.Create(ctx, rbacdomain.NewRole(constants.AdminUserRole, "管理员，拥有所有权限"))
	if err != nil {
		return 0, "", false, fmt.Errorf("创建管理员角色失败: %w", err)
	}
	return role.ID(), role.Name(), true, nil
}

func (s *SystemRoleService) ReplaceUserRoles(ctx context.Context, userID, roleID uint) error {
	role, err := s.roles.FindByID(ctx, roleID)
	if err != nil {
		return err
	}
	if role == nil {
		return rbacdomain.ErrRoleNotFound
	}
	return s.bindings.ReplaceRoles(ctx, userID, []*rbacdomain.Role{role})
}
