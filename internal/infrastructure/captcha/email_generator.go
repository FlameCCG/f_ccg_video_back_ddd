package captcha

import (
	app "f_ccg_video_back_ddd/internal/application/identity"

	"github.com/mojocn/base64Captcha"
)

// EmailGenerator 保留原有随机生成算法，将验证码库封装在基础设施中。
type EmailGenerator struct{}

var _ app.EmailCaptchaGenerator = (*EmailGenerator)(nil)

func NewEmailGenerator() *EmailGenerator { return &EmailGenerator{} }

func (*EmailGenerator) Generate(length int, charset string) (id, code string) {
	return base64Captcha.RandomId(), base64Captcha.RandText(length, charset)
}
