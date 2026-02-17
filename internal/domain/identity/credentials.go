package identity

import "strings"

// HasPassword 判断用户是否已设置登录密码（第三方首次登录可能为空）。
func (u *User) HasPassword() bool {
	if u == nil {
		return false
	}

	return strings.TrimSpace(u.Password) != ""
}

// IsThirdParty 判断是否绑定第三方登录（OpenID 非空）。
func (u *User) IsThirdParty() bool {
	if u == nil {
		return false
	}

	return strings.TrimSpace(u.OpenID) != ""
}

// RequiresOldPassword 仅当用户已设置密码时才需要校验旧密码。
//
// 第三方登录且从未设密的用户可直接设置新密码；设密后再次修改需校验旧密码。
func (u *User) RequiresOldPassword() bool {
	return u.HasPassword()
}
