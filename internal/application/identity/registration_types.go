package identity

// EmailRegisterInput 邮箱注册入参。
type EmailRegisterInput struct {
	Username     string
	Password     string
	Email        string
	RegisterIP   string
	RegisterAddr string
}
