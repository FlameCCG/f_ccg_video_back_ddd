package provisioning

import (
	"context"
	"errors"
	"fmt"

	"f_ccg_video_back_ddd/pkg/encrypt"
)

// ErrSuperUserArgsRequired 缺少用户名/密码位置参数（文案与原实现的日志一致）。
var ErrSuperUserArgsRequired = errors.New(
	"创建超级管理员失败: 请提供用户名和密码。例如: go run main.go -su admin 123456",
)

// SuperUserResult 超级管理员初始化结果，供 CLI 打印与原实现一致的日志。
type SuperUserResult struct {
	Username    string
	UserID      uint
	UserCreated bool
	RoleName    string
	RoleCreated bool
}

// SuperUserService 超级管理员初始化用例（原 flags.FlagSuperUserInit）。
type SuperUserService struct {
	repo   SuperUserRepository
	binder CasbinRoleBinder
}

// NewSuperUserService 构造超级管理员初始化用例。
func NewSuperUserService(repo SuperUserRepository, binder CasbinRoleBinder) *SuperUserService {
	return &SuperUserService{repo: repo, binder: binder}
}

// Provision 创建或更新超级管理员，并在 MySQL 与 Casbin 两侧绑定管理员角色。
//
// 步骤与原 flags.FlagSuperUserInit 完全一致：
// 取/建管理员角色 → 生成密码哈希 → 创建或更新用户（等级 6、经验 20000）
// → 覆盖用户角色关联 → Casbin 清旧角色后绑定并保存策略。
func (s *SuperUserService) Provision(ctx context.Context, args []string) (*SuperUserResult, error) {
	const requiredArgs = 2
	if len(args) < requiredArgs {
		return nil, ErrSuperUserArgsRequired
	}

	username := args[0]
	password := args[1]

	roleID, roleName, roleCreated, err := s.repo.EnsureAdminRole(ctx)
	if err != nil {
		return nil, err
	}

	hashedPassword, err := encrypt.GenPasswordHash([]byte(password))
	if err != nil {
		return nil, fmt.Errorf("生成密码哈希失败: %w", err)
	}

	userID, userCreated, err := s.repo.UpsertSuperUser(ctx, username, string(hashedPassword), roleID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.ReplaceUserRoles(ctx, userID, roleID); err != nil {
		return nil, fmt.Errorf("绑定管理员角色关联失败: %w", err)
	}

	if err := s.binder.BindUserRole(userID, roleName); err != nil {
		return nil, err
	}

	return &SuperUserResult{
		Username:    username,
		UserID:      userID,
		UserCreated: userCreated,
		RoleName:    roleName,
		RoleCreated: roleCreated,
	}, nil
}
