package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	// DefaultJSONBodyLimit 是常规 JSON 绑定的 1 MiB 请求体上限。
	DefaultJSONBodyLimit int64 = 1 << 20 // 1 MiB
	// DefaultCaptchaBodyLimit 是验证码载荷的 64 KiB 请求体上限，避免中间件为读取并复位请求体分配过多内存。
	DefaultCaptchaBodyLimit int64 = 64 << 10 // 64 KiB
)

// LimitRequestBody 在参数绑定前用 MaxBytesReader 包装请求体，读取超过上限时由读取/绑定分支处理错误。
func LimitRequestBody(c *gin.Context, limit int64) {
	if c == nil || c.Request == nil || c.Request.Body == nil || limit <= 0 {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
}
