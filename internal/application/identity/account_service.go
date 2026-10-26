package identity

import (
	"context"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/encrypt"
	"strings"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
)

// AccountService 账号凭据与角色切换用例服务
// （原 api/user_api/bind_email.go、reset_password.go、change_password.go、switch_role.go）。
type AccountService struct {
	users domain.UserAccountRepository
	roles UserRoleQuery
}

// NewAccountService 构造账号凭据服务。
func NewAccountService(users domain.UserAccountRepository, roles UserRoleQuery) *AccountService {
	return &AccountService{users: users, roles: roles}
}

// BindEmail 绑定邮箱（第三方登录用户）。
// alreadyBound 为 true 表示目标邮箱与当前邮箱相同，无需变更。
func (s *AccountService) BindEmail(ctx context.Context, in BindEmailInput) (alreadyBound bool, err error) {
	user, err := s.users.FindByID(ctx, in.UserID)
	if err != nil || user == nil {
		return false, apperr.Wrap("application/identity/account_service", domain.ErrUserNotFound, err)
	}

	if user.Email != "" && user.Email == in.Email {
		return true, nil
	}

	// 与原实现一致：统计失败时按「未占用」继续（唯一索引兜底）。
	count, _ := s.users.CountByEmailExcluding(ctx, in.Email, user.ID)
	if count > 0 {
		return false, domain.ErrEmailOccupied
	}

	if err = s.users.UpdateEmail(ctx, user.ID, in.Email); err != nil {
		return false, apperr.Wrap("application/identity/account_service", ErrBindEmailFailed, err)
	}

	return false, nil
}

// ResetPassword 通过邮箱验证码重置密码（无需登录态）。
func (s *AccountService) ResetPassword(ctx context.Context, in ResetPasswordInput) error {
	user, err := s.users.FindByEmail(ctx, in.Email)
	if err != nil {
		return apperr.Wrap("application/identity/account_service", ErrResetPasswordFailed, err)
	}

	if user == nil {
		return domain.ErrEmailNotRegistered
	}

	passwordHash, err := encrypt.GenPasswordHash([]byte(in.NewPassword))
	if err != nil {
		return apperr.Wrap("application/identity/account_service", domain.ErrPasswordHashFailed, err)
	}

	if err = s.users.UpdatePassword(ctx, user.ID, string(passwordHash)); err != nil {
		return apperr.Wrap("application/identity/account_service", ErrResetPasswordFailed, err)
	}

	return nil
}

// ChangePassword 登录态修改密码；已设密用户必须校验旧密码。
func (s *AccountService) ChangePassword(ctx context.Context, in ChangePasswordInput) error {
	user, err := s.users.FindByID(ctx, in.UserID)
	if err != nil || user == nil {
		return apperr.Wrap("application/identity/account_service", domain.ErrUserNotFound, err)
	}

	if user.RequiresOldPassword() {
		if strings.TrimSpace(in.OldPassword) == "" {
			return domain.ErrOldPasswordRequired
		}

		if !encrypt.ValidatePasswordHash(in.OldPassword, user.Password) {
			return domain.ErrOldPasswordIncorrect
		}
	}

	if user.MustChangePassword && encrypt.ValidatePasswordHash(in.NewPassword, user.Password) {
		return ErrInitialPasswordUnchanged
	}

	passwordHash, err := encrypt.GenPasswordHash([]byte(in.NewPassword))
	if err != nil {
		return apperr.Wrap("application/identity/account_service", domain.ErrPasswordHashFailed, err)
	}

	if err = s.users.UpdatePassword(ctx, user.ID, string(passwordHash)); err != nil {
		return apperr.Wrap("application/identity/account_service", ErrChangePasswordFailed, err)
	}

	return nil
}

// SwitchRole 切换当前用户的活动角色（必须已拥有该角色）。
func (s *AccountService) SwitchRole(ctx context.Context, userID, roleID uint) error {
	hasRole, err := s.roles.HasRole(ctx, userID, roleID)
	if err != nil {
		return apperr.Wrap("application/identity/account_service", ErrRoleCheckFailed, err)
	}

	if !hasRole {
		return domain.ErrRoleNotOwned
	}

	if err = s.users.UpdateCurrentRole(ctx, userID, roleID); err != nil {
		return apperr.Wrap("application/identity/account_service", ErrSwitchRoleFailed, err)
	}

	return nil
}
