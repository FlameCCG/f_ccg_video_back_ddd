package middlewares

import (
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/res"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// AIGenerationLimit reserves per-user and per-IP budgets before paid work starts.
// Failed/upstream-cancelled requests still consume an attempt to bound abuse.
func (m *Middlewares) AIGenerationLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := GetUID(c)
		if !ok {
			res.FailWithMsg("请先登录", c)
			c.Abort()

			return
		}

		if m == nil || !m.limiter.Available() {
			res.FailWithMsg("AI 服务暂不可用", c)
			c.Abort()

			return
		}

		limits := []struct {
			key    string
			count  int64
			window time.Duration
		}{
			{fmt.Sprintf("ai:user:%d:minute", uid), 6, time.Minute},
			{"ai:ip:" + c.ClientIP() + ":minute", 20, time.Minute},
			{fmt.Sprintf("ai:user:%d:day", uid), 50, 24 * time.Hour},
			{"ai:ip:" + c.ClientIP() + ":day", 150, 24 * time.Hour},
		}
		for _, limit := range limits {
			allowed, err := m.limiter.Allow(c.Request.Context(), limit.key, limit.count, limit.window)
			if err != nil {
				res.RecordFailure(apperr.Wrap("middlewares/ai_rate_limit", nil, err), c)
				res.FailWithCause(err, "AI 服务暂不可用", c)
				c.Abort()

				return
			}

			if !allowed {
				c.Header("Retry-After", "60")
				c.AbortWithStatusJSON(http.StatusTooManyRequests, res.Response{Code: res.FailServiceCode, Msg: "AI 请求额度已用完，请稍后重试", Data: gin.H{}})

				return
			}
		}

		c.Next()
	}
}
