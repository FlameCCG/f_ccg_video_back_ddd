package middlewares

import (
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestTimeout 为请求绑定统一的截止时间，避免接口无限阻塞。
func RequestTimeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := ctxutil.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
