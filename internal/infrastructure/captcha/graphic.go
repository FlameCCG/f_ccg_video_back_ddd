package captcha

import (
	app "f_ccg_video_back_ddd/internal/application/identity"

	"github.com/mojocn/base64Captcha"
)

// NewGraphicStore 提供图形验证码存储。
//
// 取代原 global.CaptchaStore 包级单例；实现仍为 base64Captcha 的默认内存存储，
// 行为（容量、过期策略）与原实现完全一致。
func NewGraphicStore() app.GraphicCaptchaStore {
	return base64Captcha.DefaultMemStore
}
