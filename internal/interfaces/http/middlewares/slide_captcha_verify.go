package middlewares

import (
	"bytes"
	"errors"
	"f_ccg_video_back_ddd/internal/infrastructure/captcha"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/res"
	"io"

	identityapp "f_ccg_video_back_ddd/internal/application/identity"

	"github.com/gin-gonic/gin"
)

type slideCaptchaVerifyReq struct {
	Token string `json:"slideCaptchaToken" form:"slideCaptchaToken"`
	X     int    `json:"slideCaptchaX" form:"slideCaptchaX"`
	Y     int    `json:"slideCaptchaY" form:"slideCaptchaY"`
}

// SlideCaptchaVerify 按登录场景滑块开关校验 token 与坐标，成功后复位请求体并继续下游绑定。
func (m *Middlewares) SlideCaptchaVerify() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.captcha.SlideCaptchaEnabled() {
			c.Next()
			return
		}

		LimitRequestBody(c, DefaultCaptchaBodyLimit)

		body, err := c.GetRawData()
		if err != nil {
			res.RecordFailure(apperr.Wrap("middlewares/slide_captcha_verify", nil, err, apperr.Validation), c)
			res.FailWithCause(err, "invalid request body", c)
			c.Abort()

			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		var req slideCaptchaVerifyReq
		if err = c.ShouldBind(&req); err != nil {
			res.RecordFailure(apperr.Wrap("middlewares/slide_captcha_verify", nil, err, apperr.Validation), c)
			res.FailWithCause(apperr.Wrap("middlewares/slide_captcha_verify", nil, err, apperr.Validation), "invalid slide captcha payload", c)
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
			"login",
		)
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
