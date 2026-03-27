package middlewares

import (
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestTimeout 为请求绑定统一的截止时间，避免接口无限阻塞。
//
// 这里只派生并传递 context，未另开 goroutine 或主动写超时响应。
// 数据库、网络和耗时循环必须协作检查该 context，才能及时结束；流式接口需使用适合长连接的预算。
func RequestTimeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := ctxutil.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// AIRequestTimeout 按请求捕获当前 AI 预算，避免固定 90 秒覆盖后台配置。
func (m *Middlewares) AIRequestTimeout() gin.HandlerFunc {
	return func(c *gin.Context) {
		seconds := 0
		if m != nil && m.cfgProvider != nil {
			if snapshot := m.cfgProvider.Get(); snapshot != nil && snapshot.AI != nil {
				seconds = snapshot.AI.TimeoutSec
			}
		}
		ctx, cancel := ctxutil.WithTimeout(c.Request.Context(), ctxutil.AITimeout(seconds))
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
