package middlewares

import (
	"bytes"
	"errors"
	"io"

	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/captcha"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type slideCaptchaVerifyReq struct {
	Token string `json:"slideCaptchaToken" form:"slideCaptchaToken"`
	X     int    `json:"slideCaptchaX" form:"slideCaptchaX"`
	Y     int    `json:"slideCaptchaY" form:"slideCaptchaY"`
}

// SlideCaptchaVerify 函数.
func (m *Middlewares) SlideCaptchaVerify() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.captcha.SlideCaptchaEnabled() {
			c.Next()
			return
		}

		LimitRequestBody(c, DefaultCaptchaBodyLimit)

		body, err := c.GetRawData()
		if err != nil {
			m.log().Error("read request body failed", zap.Error(err))
			res.FailWithMsg("invalid request body", c)
			c.Abort()

			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		var req slideCaptchaVerifyReq
		if err = c.ShouldBind(&req); err != nil {
			m.log().Error("bind slide captcha payload failed", zap.Error(err))
			res.FailWithMsg("invalid slide captcha payload", c)
			c.Abort()

			return
		}

		if req.Token == "" {
			res.FailWithMsg("slide captcha verification required", c)
			c.Abort()

			return
		}

		ok, verifyErr := m.captcha.VerifySlide(
			c.Request.Context(),
			identityapp.SlideCaptchaVerifyReq{
				Token: req.Token,
				X:     req.X,
				Y:     req.Y,
			},
		)
		if verifyErr != nil || !ok {
			switch {
			case errors.Is(verifyErr, captcha.ErrCaptchaExpired):
				res.FailWithMsg(captcha.ErrCaptchaExpired.Error(), c)
			default:
				m.log().Error("滑块验证码验证失败", zap.Error(verifyErr))
				res.FailWithMsg("验证码验证失败", c)
			}

			c.Abort()

			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}
