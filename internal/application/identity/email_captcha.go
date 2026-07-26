package identity

import (
	"context"
	"time"
)

// EmailCaptchaPayload 邮箱验证码在存储中的载荷（字段名与原 Redis JSON 完全一致）。
type EmailCaptchaPayload struct {
	Code  string `json:"code"`
	Email string `json:"email"`
}

// EmailCaptchaStore 邮箱验证码存储端口（Redis key 前缀 captcha:email:）。
type EmailCaptchaStore interface {
	// Store 写入验证码载荷并设置过期时间。
	Store(ctx context.Context, emailID, email, code string, ttl time.Duration) error
	// Consume 原子取出并删除验证码载荷；不存在时返回错误。
	Consume(ctx context.Context, emailID string) (*EmailCaptchaPayload, error)
}

// EmailCaptchaTTL 返回邮箱验证码有效期（配置 email.isExpire，单位分钟）。
func (s *CaptchaService) EmailCaptchaTTL() time.Duration {
	cfg := s.cfgProvider.Get()
	if cfg == nil || cfg.Email == nil {
		return 0
	}

	return cfg.Email.IsExpire * time.Minute
}

// StoreEmailCaptcha 写入邮箱验证码。
func (s *CaptchaService) StoreEmailCaptcha(ctx context.Context, emailID, email, code string) error {
	return s.emails.Store(ctx, emailID, email, code, s.EmailCaptchaTTL())
}

// ConsumeEmailCaptcha 取出并删除邮箱验证码载荷。
func (s *CaptchaService) ConsumeEmailCaptcha(ctx context.Context, emailID string) (*EmailCaptchaPayload, error) {
	return s.emails.Consume(ctx, emailID)
}
