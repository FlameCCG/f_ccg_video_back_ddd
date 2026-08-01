package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"

	"go.uber.org/zap"
)

// CaptchaHandler 验证码 HTTP 处理器（原 api/captcha_api.CaptchaAPI）。
type CaptchaHandler struct {
	captcha *app.CaptchaService
	emails  *app.EmailCaptchaSendService
	logger  *zap.Logger
}

// NewCaptchaHandler 构造验证码处理器。
//
// captcha 提供图形验证码存储与点击/滑块验证码用例；
// emails 提供邮箱验证码发送用例（内含限流、邮箱可达性校验与异步投递）。
func NewCaptchaHandler(
	captcha *app.CaptchaService,
	emails *app.EmailCaptchaSendService,
	logger *zap.Logger,
) *CaptchaHandler {
	return &CaptchaHandler{captcha: captcha, emails: emails, logger: loggerOrNop(logger)}
}

func (h *CaptchaHandler) log() *zap.Logger {
	if h == nil {
		return zap.NewNop()
	}

	return loggerOrNop(h.logger)
}
