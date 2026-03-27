package identity

import (
	"context"

	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/pkg/encrypt"

	"go.uber.org/zap"
)

// AccountService 账号凭据与角色切换用例服务。
type AccountService struct {
	txm      tx.Manager
	users    identitydomain.UserAccountRepository
	roles    UserRoleQuery
	sessions SessionStore
	logger   *zap.Logger
}

// NewAccountService 构造账号凭据服务。
func NewAccountService(txm tx.Manager, users identitydomain.UserAccountRepository, roles UserRoleQuery, sessions SessionStore, logger *zap.Logger) *AccountService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AccountService{txm: txm, users: users, roles: roles, sessions: sessions, logger: logger}
}

// BindEmail 绑定邮箱（第三方登录用户）。
// alreadyBound 为 true 表示目标邮箱与当前邮箱相同，无需变更。
func (s *AccountService) BindEmail(ctx context.Context, in BindEmailInput) (alreadyBound bool, err error) {
	err = s.transact(ctx, ErrBindEmailFailed, func(txCtx context.Context) error {
		user, findErr := s.users.FindByIDForUpdate(txCtx, in.UserID)
		if findErr != nil || user == nil {
			return identitydomain.ErrUserNotFound
		}
		if user.EmailMatches(in.Email) {
			alreadyBound = true
			return nil
		}
		// 保留统计失败按未占用继续、唯一索引兜底的既有语义。
		count, _ := s.users.CountByEmailExcluding(txCtx, in.Email, user.ID())
		var bindErr error
		alreadyBound, bindErr = user.BindEmail(in.Email, count)
		if bindErr != nil {
			return bindErr
		}
		if saveErr := s.users.SaveEmail(txCtx, user); saveErr != nil {
			return ErrBindEmailFailed
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return alreadyBound, nil
}

// ResetPassword 通过邮箱验证码重置密码（无需登录态）。
func (s *AccountService) ResetPassword(ctx context.Context, in ResetPasswordInput) error {
	var userID uint
	var sessionIDs []string
	err := s.transact(ctx, ErrResetPasswordFailed, func(txCtx context.Context) error {
		user, err := s.users.FindByEmailForUpdate(txCtx, in.Email)
		if err != nil {
			return ErrResetPasswordFailed
		}
		if user == nil {
			return identitydomain.ErrEmailNotRegistered
		}
		passwordHash, err := encrypt.GenPasswordHash([]byte(in.NewPassword))
		if err != nil {
			return identitydomain.ErrPasswordHashFailed
		}
		if err := user.ResetPassword(string(passwordHash)); err != nil {
			return err
		}
		sessionIDs = s.snapshotCredentialSessions(txCtx, user.ID())
		if err := s.users.SaveCredentials(txCtx, user); err != nil {
			return ErrResetPasswordFailed
		}
		userID = user.ID()
		return nil
	})
	return s.finishCredentialChange(ctx, userID, sessionIDs, err)
}

// ChangePassword 登录态修改密码；已设密用户必须校验旧密码。
func (s *AccountService) ChangePassword(ctx context.Context, in ChangePasswordInput) error {
	var userID uint
	var sessionIDs []string
	err := s.transact(ctx, ErrChangePasswordFailed, func(txCtx context.Context) error {
		user, err := s.users.FindByIDForUpdate(txCtx, in.UserID)
		if err != nil || user == nil {
			return identitydomain.ErrUserNotFound
		}
		change := identitydomain.PasswordChange{
			OldPassword:        in.OldPassword,
			OldPasswordMatches: encrypt.ValidatePasswordHash(in.OldPassword, user.Password()),
		}
		// 先校验旧密码，再计算新哈希，以此确定错误优先级。
		if err := user.CheckPasswordChange(change); err != nil {
			return err
		}
		passwordHash, err := encrypt.GenPasswordHash([]byte(in.NewPassword))
		if err != nil {
			return identitydomain.ErrPasswordHashFailed
		}
		change.PasswordHash = string(passwordHash)
		if err := user.ChangePassword(change); err != nil {
			return err
		}
		sessionIDs = s.snapshotCredentialSessions(txCtx, user.ID())
		if err := s.users.SaveCredentials(txCtx, user); err != nil {
			return ErrChangePasswordFailed
		}
		userID = user.ID()
		return nil
	})
	return s.finishCredentialChange(ctx, userID, sessionIDs, err)
}

// 在提交新密码前固定清理集合；提交后建立的新版本会话不属于该集合。
func (s *AccountService) snapshotCredentialSessions(ctx context.Context, userID uint) []string {
	if s.sessions == nil {
		return nil
	}
	ids, err := s.sessions.SnapshotUser(ctx, userID)
	if err != nil {
		s.logger.Warn("password session snapshot failed", zap.Uint("user_id", userID), zap.Error(err))
		return nil
	}
	return ids
}

func (s *AccountService) finishCredentialChange(ctx context.Context, userID uint, sessionIDs []string, transactionErr error) error {
	if transactionErr != nil {
		return transactionErr
	}
	if s.sessions != nil && len(sessionIDs) > 0 {
		// 密码已提交；即使清理失败，强制从 DB 校验 pwdVer 也会拒绝全部旧凭据。
		if err := s.sessions.RevokeSessions(context.WithoutCancel(ctx), userID, sessionIDs); err != nil {
			s.logger.Warn("password changed but session cleanup failed", zap.Uint("user_id", userID), zap.Error(err))
		}
	}
	return nil
}

// SwitchRole 切换当前用户的活动角色（必须已拥有该角色）。
func (s *AccountService) SwitchRole(ctx context.Context, userID, roleID uint) error {
	return s.transact(ctx, ErrSwitchRoleFailed, func(txCtx context.Context) error {
		hasRole, err := s.roles.HasRole(txCtx, userID, roleID)
		if err != nil {
			return ErrRoleCheckFailed
		}
		if err := (identitydomain.RoleAccess{}).EnsureOwned(hasRole); err != nil {
			return err
		}
		user, err := s.users.FindByIDForUpdate(txCtx, userID)
		if err != nil {
			return ErrSwitchRoleFailed
		}
		// 更新不存在的行时不执行写入，也不额外校验账号是否存在。
		if user == nil {
			return nil
		}
		if err := user.SwitchRole(roleID, hasRole); err != nil {
			return err
		}
		if err := s.users.SaveCurrentRole(txCtx, user); err != nil {
			return ErrSwitchRoleFailed
		}
		return nil
	})
}

// transact 保留领域/用例错误；开启或提交事务失败仍返回该用例已有的失败文案。
func (s *AccountService) transact(ctx context.Context, failure error, fn func(context.Context) error) error {
	var actionErr error
	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		actionErr = fn(txCtx)
		return actionErr
	})
	if actionErr != nil {
		return actionErr
	}
	if err != nil {
		return failure
	}
	return nil
}
