package identity

// EmailCaptchaGenerator 提供随机标识和验证码；长度及字符集由用例策略指定。
type EmailCaptchaGenerator interface {
	Generate(length int, charset string) (id, code string)
}

// EmailCaptchaSender 邮箱验证码投递端口。
// captchaType 与接口层一致：1 注册 / 2 重置密码 / 3 绑定邮箱。
type EmailCaptchaSender interface {
	SendEmailCaptcha(captchaType int, email, code string) error
}

// EmailVerifyResult 邮箱可达性校验结果。
type EmailVerifyResult struct {
	// SyntaxValid 语法是否合法。
	SyntaxValid bool
	// HasSMTPResult 是否拿到了 SMTP 探测结果（对应 ret.SMTP != nil）。
	HasSMTPResult bool
	// SMTPHostExists SMTP 主机是否存在。
	SMTPHostExists bool
	// HasMXRecords 域名是否有 MX 记录。
	HasMXRecords bool
}

// EmailAddressVerifier 邮箱地址可达性校验端口，通过依赖注入提供。
type EmailAddressVerifier interface {
	Verify(email string) (*EmailVerifyResult, error)
}
