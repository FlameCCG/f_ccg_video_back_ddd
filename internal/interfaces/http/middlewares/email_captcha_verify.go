package middlewares

import (
	"bytes"
	"io"
	"strings"
	"time"

	"f_ccg_video_back_ddd/pkg/constants"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	// emailCaptchaVerifyLimit 单个维度（emailID / email / IP）在窗口内允许的校验次数。
	emailCaptchaVerifyLimit int64 = 10
	// emailCaptchaVerifyWindow 校验限流窗口。
	emailCaptchaVerifyWindow = 10 * time.Minute
)

// EmailCaptchaVerifyReq 类型.
type EmailCaptchaVerifyReq struct {
	EmailID   string `json:"emailID"`
	EmailCode string `json:"emailCode"`
	Email     string `json:"email"`
}

// EmailCaptchaVerify 函数.
func (m *Middlewares) EmailCaptchaVerify() gin.HandlerFunc {
	return m.emailCaptchaVerify(false)
}

// EmailCaptchaVerifyRequired 函数.
func (m *Middlewares) EmailCaptchaVerifyRequired() gin.HandlerFunc {
	return m.emailCaptchaVerify(true)
}

func (m *Middlewares) emailCaptchaVerify(force bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !force && !m.captcha.EmailCaptchaEnabled() {
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

		var req EmailCaptchaVerifyReq
		if err = c.ShouldBindJSON(&req); err != nil {
			m.log().Error("bind email captcha payload failed", zap.Error(err))
			res.FailWithError(err, c)
			c.Abort()

			return
		}

		if ok, rateErr := m.allowEmailCaptchaVerify(c, req); rateErr != nil || !ok {
			if rateErr != nil {
				m.log().Warn("email captcha verify rate limit failed", zap.Error(rateErr))
			}

			res.FailWithMsg("验证码验证过于频繁，请稍后再试", c)
			c.Abort()

			return
		}

		payload, err := m.captcha.ConsumeEmailCaptcha(c.Request.Context(), req.EmailID)
		if err != nil {
			res.FailWithMsg("email captcha expired or missing", c)
			c.Abort()

			return
		}

		if payload.Email != req.Email {
			m.log().Error("email captcha mismatch", zap.String("captchaEmail", payload.Email), zap.String("requestEmail", req.Email))
			res.FailWithMsg("email captcha mismatch", c)
			c.Abort()

			return
		}

		if payload.Code != req.EmailCode {
			res.FailWithMsg("email captcha verification failed", c)
			c.Abort()

			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}

func (m *Middlewares) allowEmailCaptchaVerify(c *gin.Context, req EmailCaptchaVerifyReq) (bool, error) {
	keys := []string{
		constants.EmailCaptchaVerifyRateKeyPrefix + "id:" + strings.TrimSpace(req.EmailID),
		constants.EmailCaptchaVerifyRateKeyPrefix + "email:" + strings.ToLower(strings.TrimSpace(req.Email)),
		constants.EmailCaptchaVerifyRateKeyPrefix + "ip:" + strings.TrimSpace(c.ClientIP()),
	}
	for _, key := range keys {
		ok, err := m.limiter.Allow(c.Request.Context(), key, emailCaptchaVerifyLimit, emailCaptchaVerifyWindow)
		if err != nil || !ok {
			return ok, err
		}
	}

	return true, nil
}
