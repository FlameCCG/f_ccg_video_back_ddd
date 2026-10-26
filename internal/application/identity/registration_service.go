package identity

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/constants"
	"f_ccg_video_back_ddd/pkg/encrypt"
	"f_ccg_video_back_ddd/pkg/logctx"

	domain "f_ccg_video_back_ddd/internal/domain/identity"

	"go.uber.org/zap"
)

// RegistrationService 邮箱注册用例服务（原 api/user_api/register.go）。
type RegistrationService struct {
	txm    tx.Manager
	users  domain.UserAccountRepository
	roles  UserRoleQuery
	binder UserRoleBinder
	logger *zap.Logger
}

// NewRegistrationService 构造邮箱注册服务。
func NewRegistrationService(
	txm tx.Manager,
	users domain.UserAccountRepository,
	roles UserRoleQuery,
	binder UserRoleBinder,
	logger *zap.Logger,
) *RegistrationService {
	return &RegistrationService{txm: txm, users: users, roles: roles, binder: binder, logger: logger}
}

// EmailRegister 邮箱注册：写用户 + 默认角色关联（同一事务），提交后再绑定 Casbin 角色。
//
// 前置占用查询只改善提示，数据库唯一约束才裁定并发注册冲突。
// Casbin 绑定是提交后的另一步；失败后的删除属于尽力补偿，补偿自身也可能失败，并非跨系统原子回滚。
//
// 事务边界与补偿顺序与原实现一致：Casbin 写在 Commit 之后，绑定失败则补偿删除用户，
// 避免 Commit 失败留下幽灵权限或无角色的半成品账号。
func (s *RegistrationService) EmailRegister(ctx context.Context, in EmailRegisterInput) error {
	exists, err := s.users.ExistsByUsernameOrEmail(ctx, in.Username, in.Email)
	if err != nil {
		// 原实现只在「查到记录」时拦截，查询报错会继续走注册流程
		// （唯一索引兜底 → 落库失败回「注册失败」）。此处保持同样语义。
		logctx.With(ctx, s.logger).Warn("查询用户名/邮箱占用失败", zap.String("event", "identity.registration_service.email_register.degraded"), zap.String("operation", "identity.registration_service.email_register"), zap.Error(err))
	}

	if exists {
		return domain.ErrUserOrEmailExists
	}

	// bcrypt 放在事务外，避免长时间占用连接。
	passwordHash, err := encrypt.GenPasswordHash([]byte(in.Password))
	if err != nil {
		return apperr.Wrap("application/identity/registration_service", domain.ErrPasswordHashFailed, err)
	}

	user := &domain.User{
		Username:       in.Username,
		Password:       string(passwordHash),
		Email:          in.Email,
		RegisterIP:     in.RegisterIP,
		RegisterAddr:   in.RegisterAddr,
		RegisterSource: domain.RegisterSourceEmail,
		Gender:         enum.GenderUnknown,
	}

	// 用户创建钩子需要读取「注册默认横幅 ID」，必须在开启事务前包装 ctx。
	createCtx := s.users.WithDefaultBanner(ctx)

	if err = s.txm.Do(createCtx, func(txCtx context.Context) error {
		roleID, roleErr := s.roles.RoleIDByName(txCtx, constants.CommonUserRole)
		if roleErr != nil {
			return roleErr
		}

		user.CurrentRoleID = roleID

		if createErr := s.users.Create(txCtx, user, roleID); createErr != nil {
			return createErr
		}

		return s.users.InitializeDefaults(txCtx, user.ID)
	}); err != nil {
		return apperr.Wrap("application/identity/registration_service", ErrRegisterFailed, err)
	}

	// Casbin 在 Commit 之后写入，避免 Commit 失败留下幽灵权限。
	if err = s.binder.AddRoleForUser(user.ID, constants.CommonUserRole); err != nil {
		// 补偿：清理已提交用户，避免无角色半成品账号。
		if delErr := s.users.DeleteByID(ctx, user.ID); delErr != nil {
			err = errors.Join(err, delErr)
		}

		return apperr.Wrap("application/identity/registration_service", ErrRegisterFailed, err)
	}

	return nil
}
