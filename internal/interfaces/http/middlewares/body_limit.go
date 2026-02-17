package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	// DefaultJSONBodyLimit 常量.
	DefaultJSONBodyLimit int64 = 1 << 20 // 1 MiB
	// DefaultCaptchaBodyLimit 常量.
	DefaultCaptchaBodyLimit int64 = 64 << 10 // 64 KiB
)

// LimitRequestBody wraps the request body with MaxBytesReader before binding.
func LimitRequestBody(c *gin.Context, limit int64) {
	if c == nil || c.Request == nil || c.Request.Body == nil || limit <= 0 {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
}
