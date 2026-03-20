package identity

// BindEmailInput 绑定邮箱入参。
type BindEmailInput struct {
	UserID uint
	Email  string
}

// ResetPasswordInput 邮箱验证码重置密码入参。
type ResetPasswordInput struct {
	Email       string
	NewPassword string
}

// ChangePasswordInput 登录态修改密码入参。
type ChangePasswordInput struct {
	UserID      uint
	OldPassword string
	NewPassword string
}
