package captcha

import (
	app "f_ccg_video_back_ddd/internal/application/identity"

	"github.com/mojocn/base64Captcha"
)

// NewGraphicStore 提供图形验证码存储。
//
// 取代原 global.CaptchaStore 与依赖库 DefaultMemStore 两层包级单例；
// 每个应用实例得到独立存储，容量与过期策略仍使用依赖库默认值。
func NewGraphicStore() app.GraphicCaptchaStore {
	return base64Captcha.NewMemoryStore(base64Captcha.GCLimitNumber, base64Captcha.Expiration)
}
