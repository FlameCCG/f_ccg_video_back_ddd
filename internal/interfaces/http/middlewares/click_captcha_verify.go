package middlewares

import (
	"bytes"
	"encoding/json"
	"errors"
	"f_ccg_video_back_ddd/internal/infrastructure/captcha"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/res"
	"io"

	identityapp "f_ccg_video_back_ddd/internal/application/identity"

	"github.com/gin-gonic/gin"
)

type clickCaptchaVerifyReq struct {
	Token string                          `json:"captchaToken"`
	Dots  []identityapp.ClickCaptchaPoint `json:"captchaDots"`
}

// ClickCaptchaVerify 按登录场景点击验证码开关校验交互结果，失败终止中间件链。
func (m *Middlewares) ClickCaptchaVerify() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.captcha.ClickCaptchaEnabled() {
			c.Next()
			return
		}

		LimitRequestBody(c, DefaultCaptchaBodyLimit)

		body, err := c.GetRawData()
		if err != nil {
			res.RecordFailure(apperr.Wrap("middlewares/click_captcha_verify", nil, err, apperr.Validation), c)
			res.FailWithCause(err, "invalid request body", c)
			c.Abort()

			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		var req clickCaptchaVerifyReq
		if err = json.Unmarshal(body, &req); err != nil {
			res.RecordFailure(apperr.Wrap("middlewares/click_captcha_verify", nil, err, apperr.Validation), c)
			res.FailWithCause(apperr.Wrap("middlewares/click_captcha_verify", nil, err, apperr.Validation), "invalid captcha payload", c)
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
			if verifyErr != nil {
				res.RecordFailure(verifyErr, c)
			}

			switch {
			case errors.Is(verifyErr, captcha.ErrCaptchaExpired):
				res.FailWithMsg(captcha.ErrCaptchaExpired.Error(), c)
			default:
				if verifyErr != nil {
					res.RecordFailure(verifyErr, c)
				} else {
					res.RecordFailure(apperr.Wrap("captcha.verify", nil, nil, apperr.Rejected), c)
				}

				res.FailWithMsg("验证码验证失败", c)
			}

			c.Abort()

			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}
