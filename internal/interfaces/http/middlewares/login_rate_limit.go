package middlewares

import (
	"bytes"
	"encoding/json"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/constants"
	"f_ccg_video_back_ddd/pkg/res"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// loginRateIPLimit 单 IP 在窗口内允许的登录尝试次数（宽松于账号维度，容忍 NAT 出口共用）。
	loginRateIPLimit int64 = 30
	// loginRateAccountLimit 同一「IP + 账号」在窗口内允许的登录尝试次数。
	loginRateAccountLimit int64 = 10
	// loginRateWindow 登录限流窗口。
	loginRateWindow = 10 * time.Minute
)

// LoginRateLimit 登录防爆破限流：按 IP 维度计数；请求体携带 username（密码登录）时
// 叠加「IP + 账号」维度。
//
// 账号维度必须绑定 IP：若全局按账号计数，任何知道 username 的人发满阈值即可
// 让该账号在所有网络位置都登录不上（定向锁号）。绑定后攻击者只能耗尽自己 IP
// 上的额度，受害者从其他网络登录不受影响；分布式爆破则由 IP 维度、验证码与
// 密码强度兜底。
//
// 独立于验证码开关——验证码被关闭或其服务异常时仍提供兜底防护。Redis 异常时
// 与邮箱验证码限流保持一致：拒绝请求并记录告警，避免限流组件失效后零防护。
func (m *Middlewares) LoginRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		LimitRequestBody(c, DefaultCaptchaBodyLimit)

		clientIP := c.ClientIP()

		if !m.allowLoginRate(c, constants.LoginRateKeyPrefix+"ip:"+clientIP, loginRateIPLimit) {
			return
		}

		if username := loginUsernameFromBody(c); username != "" {
			key := constants.LoginRateKeyPrefix + "acct:" + clientIP + ":" + username
			if !m.allowLoginRate(c, key, loginRateAccountLimit) {
				return
			}
		}

		c.Next()
	}
}

// allowLoginRate 检查单个维度的登录限流，超限或 Redis 异常时终结请求。
func (m *Middlewares) allowLoginRate(c *gin.Context, key string, limit int64) bool {
	ok, err := m.limiter.Allow(c.Request.Context(), key, limit, loginRateWindow)
	if err != nil {
		res.RecordFailure(apperr.Wrap("middlewares/login_rate_limit", nil, err), c)
		res.FailWithCause(err, "登录服务繁忙，请稍后再试", c)
		c.Abort()

		return false
	}

	if !ok {
		res.FailWithMsg("登录尝试过于频繁，请稍后再试", c)
		c.Abort()

		return false
	}

	return true
}

// loginUsernameFromBody 尽力从 JSON 请求体提取 username（密码登录）。
//
// 提取失败（非 JSON、OAuth 请求体等）不视为错误，退化为纯 IP 维度限流；
// 无论成败都会把 body 复位，供后续验证码/绑定中间件读取。
func loginUsernameFromBody(c *gin.Context) string {
	if c.Request == nil || c.Request.Body == nil {
		return ""
	}

	body, err := c.GetRawData()
	if err != nil {
		return ""
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	var payload struct {
		UserName string `json:"username"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}

	return strings.ToLower(strings.TrimSpace(payload.UserName))
}
