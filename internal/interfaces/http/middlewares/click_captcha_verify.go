package middlewares

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/captcha"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type clickCaptchaVerifyReq struct {
	Token string                          `json:"captchaToken"`
	Dots  []identityapp.ClickCaptchaPoint `json:"captchaDots"`
}

// ClickCaptchaVerify 函数.
func (m *Middlewares) ClickCaptchaVerify() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.captcha.ClickCaptchaEnabled() {
			c.Next()
			return
		}

		LimitRequestBody(c, DefaultCaptchaBodyLimit)

		body, err := c.GetRawData()
		if err != nil {
			zap.L().Error("read request body failed", zap.Error(err))
			res.FailWithMsg("invalid request body", c)
			c.Abort()

			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		var req clickCaptchaVerifyReq
		if err = json.Unmarshal(body, &req); err != nil {
			zap.L().Error("unmarshal click captcha payload failed", zap.Error(err))
			res.FailWithMsg("invalid captcha payload", c)
			c.Abort()

			return
		}

		if req.Token == "" || len(req.Dots) == 0 {
			res.FailWithMsg("captcha verification required", c)
			c.Abort()

			return
		}

		ok, verifyErr := m.captcha.VerifyClick(c.Request.Context(), req.Token, req.Dots)
		if verifyErr != nil || !ok {
			switch {
			case errors.Is(verifyErr, captcha.ErrCaptchaExpired):
				res.FailWithMsg(captcha.ErrCaptchaExpired.Error(), c)
			default:
				zap.L().Error("点击验证码验证失败", zap.Error(verifyErr))
				res.FailWithMsg("验证码验证失败", c)
			}

			c.Abort()

			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}
