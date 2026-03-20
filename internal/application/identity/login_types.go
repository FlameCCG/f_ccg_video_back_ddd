package identity

// PwdLoginInput 用户名/邮箱 + 密码登录入参。
type PwdLoginInput struct {
	Identifier string
	Password   string
	IP         string
	Addr       string
	UserAgent  string
}
