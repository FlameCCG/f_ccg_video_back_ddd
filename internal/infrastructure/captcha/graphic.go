package captcha

import (
	app "f_ccg_video_back_ddd/internal/application/identity"

	"github.com/mojocn/base64Captcha"
)

// NewGraphicStore 提供图形验证码存储。
//
// 每个应用实例拥有独立存储，容量与过期策略使用依赖库默认值。
func NewGraphicStore() app.GraphicCaptchaStore {
	return base64Captcha.NewMemoryStore(base64Captcha.GCLimitNumber, base64Captcha.Expiration)
}
