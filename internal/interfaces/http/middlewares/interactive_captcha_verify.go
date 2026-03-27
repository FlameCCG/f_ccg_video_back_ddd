package middlewares

import (
	"bytes"
	"context"
	"errors"
	"io"

	captcha_service "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/internal/infrastructure/captcha"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

type interactiveCaptchaReq struct {
	ClickToken string                              `json:"captchaToken" form:"captchaToken"`
	Dots       []captcha_service.ClickCaptchaPoint `json:"captchaDots" form:"captchaDots"`
	SlideToken string                              `json:"slideCaptchaToken" form:"slideCaptchaToken"`
	X          int                                 `json:"slideCaptchaX" form:"slideCaptchaX"`
	Y          int                                 `json:"slideCaptchaY" form:"slideCaptchaY"`
}

type interactiveCaptchaVerifier interface {
	VerifyClick(context.Context, string, []captcha_service.ClickCaptchaPoint) (bool, error)
	VerifySlide(context.Context, captcha_service.SlideCaptchaVerifyReq) (bool, error)
}

type scopedCaptchaVerifier struct {
	service *captcha_service.CaptchaService
	scene   string
}

func (v scopedCaptchaVerifier) VerifyClick(ctx context.Context, token string, dots []captcha_service.ClickCaptchaPoint) (bool, error) {
	return v.service.VerifyClick(ctx, token, dots, v.scene)
}

func (v scopedCaptchaVerifier) VerifySlide(ctx context.Context, req captcha_service.SlideCaptchaVerifyReq) (bool, error) {
	return v.service.VerifySlide(ctx, req, v.scene)
}

// InteractiveCaptchaVerify 按场景校验一种已开启的验证码。
func (m *Middlewares) InteractiveCaptchaVerify(scene string) gin.HandlerFunc {
	return func(c *gin.Context) {
		interactiveCaptchaVerify(func() site.CaptchaOptions {
			cfg := m.cfgProvider.Get()
			if cfg == nil {
				return site.CaptchaOptions{}
			}
			return cfg.Site.InteractiveCaptchaOptions(scene)
		}, scopedCaptchaVerifier{service: m.captcha, scene: scene})(c)
	}
}

func interactiveCaptchaVerify(options func() site.CaptchaOptions, verifier interactiveCaptchaVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		enabled := options()
		if !enabled.Click && !enabled.Slide {
			c.Next()
			return
		}

		LimitRequestBody(c, DefaultCaptchaBodyLimit)
		body, err := c.GetRawData()
		if err != nil {
			res.FailWithMsg("invalid request body", c)
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		var req interactiveCaptchaReq
		if err = c.ShouldBind(&req); err != nil {
			captchaFailure(c, "interactive", "invalid", "交互验证码参数错误，请重新验证")
			return
		}

		var ok bool
		kind, label := "interactive", "交互验证码"
		// 必须且只能提交一种已开启的验证码，不能通过缺字段或关闭的类型绕过校验。
		switch {
		case enabled.Click && req.ClickToken != "" && req.SlideToken == "" && len(req.Dots) > 0:
			kind, label = "click", "文字点选验证码"
			ok, err = verifier.VerifyClick(c.Request.Context(), req.ClickToken, req.Dots)
		case enabled.Slide && req.SlideToken != "" && req.ClickToken == "":
			kind, label = "slide", "滑块验证码"
			ok, err = verifier.VerifySlide(c.Request.Context(), captcha_service.SlideCaptchaVerifyReq{
				Token: req.SlideToken, X: req.X, Y: req.Y,
			})
		default:
			if enabled.Click && !enabled.Slide {
				kind, label = "click", "文字点选验证码"
			}
			if enabled.Slide && !enabled.Click {
				kind, label = "slide", "滑块验证码"
			}
			captchaFailure(c, kind, "required", "请完成"+label)
			return
		}
		if err != nil || !ok {
			reason, message := "invalid", label+"错误，请重新验证"
			if errors.Is(err, captcha.ErrCaptchaExpired) {
				reason, message = "expired", label+"已过期或不存在，请重新验证"
			} else if err != nil {
				reason, message = "unavailable", label+"暂时不可用，请重试"
			}
			captchaFailure(c, kind, reason, message)
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}
