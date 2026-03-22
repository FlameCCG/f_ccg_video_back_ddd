package identity

import (
	"errors"
	"time"
)

const (
	EmailCaptchaRegister         = 1
	EmailCaptchaPasswordRecovery = 2
	EmailCaptchaBinding          = 3
)

var (
	ErrEmailSyntaxInvalid             = errors.New("邮箱格式错误")
	ErrEmailHostNotExists             = errors.New("邮箱服务器不存在")
	ErrEmailNoMXRecords               = errors.New("邮箱服务器无MX记录")
	ErrEmailAlreadyRegistered         = errors.New("该邮箱已注册")
	ErrEmailCaptchaEmailNotRegistered = errors.New("该邮箱未注册")
	ErrEmailAlreadyBound              = errors.New("该邮箱已被绑定")
)

type EmailAddressFacts struct{ SyntaxValid, HasSMTPResult, SMTPHostExists, HasMXRecords bool }

func (f EmailAddressFacts) EnsureReachable() error {
	if !f.SyntaxValid {
		return ErrEmailSyntaxInvalid
	}
	if f.HasSMTPResult && !f.SMTPHostExists {
		return ErrEmailHostNotExists
	}
	if !f.HasMXRecords {
		return ErrEmailNoMXRecords
	}
	return nil
}

type EmailCaptchaPolicy struct{}
type CaptchaSendPolicy struct {
	CodeLength      int
	Charset         string
	Window          time.Duration
	PerEmail, PerIP int64
}

func (EmailCaptchaPolicy) Sending() CaptchaSendPolicy {
	return CaptchaSendPolicy{CodeLength: 5, Charset: "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", Window: 10 * time.Minute, PerEmail: 5, PerIP: 20}
}

// EnsureAccount 保留查询失败时注册/绑定放行、重置密码按未注册处理的区别。
func (EmailCaptchaPolicy) EnsureAccount(purpose int, found, lookupSucceeded bool) error {
	switch purpose {
	case EmailCaptchaRegister:
		if lookupSucceeded && found {
			return ErrEmailAlreadyRegistered
		}
	case EmailCaptchaPasswordRecovery:
		if !lookupSucceeded || !found {
			return ErrEmailCaptchaEmailNotRegistered
		}
	case EmailCaptchaBinding:
		if lookupSucceeded && found {
			return ErrEmailAlreadyBound
		}
	}
	return nil
}
