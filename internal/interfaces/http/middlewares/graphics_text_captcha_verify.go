package middlewares

import (
	"bytes"
	"io"
	"strings"

	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CaptchaMiddlewareRequest 是图形验证码中间件读取的局部载荷，不包含后续登录/注册请求的全部字段。
type CaptchaMiddlewareRequest struct {
	CaptchaID   string `form:"captchaID" json:"captchaID"`
	CaptchaCode string `form:"captchaCode" json:"captchaCode"`
}

// GraphicsTextCaptchaMiddleware 按图形验证码开关校验当前请求，读取请求体后复位以供后续参数绑定继续使用。
func (m *Middlewares) GraphicsTextCaptchaMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.captcha.GraphicCaptchaEnabled() {
			c.Next()
			return
		}

		LimitRequestBody(c, DefaultCaptchaBodyLimit)

		body, err := c.GetRawData()
		if err != nil {
			m.log().Error("read request body failed", zap.Error(err))
			res.FailWithMsg("请求体无效", c)
			c.Abort()

			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		var payload CaptchaMiddlewareRequest
		if err = c.ShouldBind(&payload); err != nil {
			m.log().Error("bind graphics captcha payload failed", zap.Error(err))
			res.FailWithMsg("图形验证码参数错误", c)
			c.Abort()

			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		captchaID := strings.TrimSpace(payload.CaptchaID)
		captchaCode := strings.TrimSpace(payload.CaptchaCode)
		if captchaID == "" || captchaCode == "" {
			res.FailWithMsg("请填写图形验证码", c)
			c.Abort()

			return
		}

		if ok := m.captcha.VerifyGraphic(captchaID, captchaCode); !ok {
			m.log().Error("graphics captcha verify failed")
			res.FailWithMsg("图形验证码错误", c)
			c.Abort()

			return
		}

		c.Next()
	}
}
