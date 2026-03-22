package identity

import "strings"

// PasswordChange 携带当次密码验证事实及新哈希。散列计算由应用层完成。
type PasswordChange struct {
	OldPassword        string
	OldPasswordMatches bool
	NewPasswordMatches bool
	PasswordHash       string
}

// EnsureCanLogin 对密码登录、OAuth 和会话刷新采用相同的封禁资格。
func (u *User) EnsureCanLogin() error {
	if u == nil {
		return ErrUserNotFound
	}
	if u.IsBanned() {
		return ErrAccountBanned
	}
	return nil
}

func (u *User) EnsurePasswordLogin(passwordMatches bool) error {
	if u == nil || !passwordMatches {
		return ErrCredentialsInvalid
	}
	return u.EnsureCanLogin()
}

type RegistrationPolicy struct{}

func (RegistrationPolicy) EnsureAvailable(accountExists bool) error {
	if accountExists {
		return ErrUserOrEmailExists
	}
	return nil
}

func (u *User) EnsureAdminLogin(ownsAdminRole bool) error {
	if !ownsAdminRole {
		return ErrNotAdminAccount
	}
	return nil
}

// RoleAccess 解释外层取得的角色名事实，不依赖权限上下文的实体。
type RoleAccess struct{}

func (RoleAccess) EnsureOwned(owned bool) error {
	if !owned {
		return ErrRoleNotOwned
	}
	return nil
}

func (RoleAccess) HasRole(names []string, required string) bool {
	for _, name := range names {
		if name == required {
			return true
		}
	}
	return false
}

// CheckPasswordChange 保持先检查旧密码、再检查初始密码复用的错误顺序。
func (u *User) CheckPasswordChange(change PasswordChange) error {
	if u == nil {
		return ErrUserNotFound
	}
	if err := u.CheckOldPassword(change.OldPassword, func(string) bool { return change.OldPasswordMatches }); err != nil {
		return err
	}
	if u.ReusesInitialPassword(change.NewPasswordMatches) {
		return ErrInitialPasswordUnchanged
	}
	return nil
}

// ChangePassword 在资格校验成功后一起更新密码、强制改密标记和刷新凭据。
func (u *User) ChangePassword(change PasswordChange) error {
	if err := u.CheckPasswordChange(change); err != nil {
		return err
	}
	return u.ResetPassword(change.PasswordHash)
}

// ResetPassword 使用已验证的邮箱恢复资格重置凭据，不要求提供旧密码。
func (u *User) ResetPassword(passwordHash string) error {
	if u == nil {
		return ErrUserNotFound
	}
	u.password = passwordHash
	u.refreshTokenHash = ""
	u.mustChangePassword = false
	return nil
}

// BindEmail 按已有占用事实绑定邮箱；同一邮箱保持幂等，不增加格式限制。
func (u *User) BindEmail(email string, occupied int64) (bool, error) {
	if u == nil {
		return false, ErrUserNotFound
	}
	if u.EmailMatches(email) {
		return true, nil
	}
	if occupied > 0 {
		return false, ErrEmailOccupied
	}
	u.email = email
	return false, nil
}

// SwitchRole 仅允许选择当次事实中已经拥有的角色。
func (u *User) SwitchRole(roleID uint, owned bool) error {
	if err := (RoleAccess{}).EnsureOwned(owned); err != nil {
		return err
	}
	if u == nil {
		return ErrUserNotFound
	}
	u.currentRoleID = roleID
	return nil
}

// HasPassword 判断用户是否已设置登录密码（第三方首次登录可能为空）。
func (u *User) HasPassword() bool {
	if u == nil {
		return false
	}

	return strings.TrimSpace(u.password) != ""
}

// IsThirdParty 判断是否绑定第三方登录（OpenID 非空）。
func (u *User) IsThirdParty() bool {
	if u == nil {
		return false
	}

	return strings.TrimSpace(u.openID) != ""
}

// RequiresOldPassword 仅当用户已设置密码时才需要校验旧密码。
//
// 第三方登录且从未设密的用户可直接设置新密码；设密后再次修改需校验旧密码。
func (u *User) RequiresOldPassword() bool {
	return u.HasPassword()
}
