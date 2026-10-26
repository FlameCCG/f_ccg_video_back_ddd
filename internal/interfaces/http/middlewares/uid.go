package middlewares

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetUID 从 gin.Context 安全读取当前登录用户 ID。
// JWT 中间件写入的 uid 为 string；部分测试/调用可能写入数值类型。
func GetUID(c *gin.Context) (uint, bool) {
	if c == nil {
		return 0, false
	}

	val, ok := c.Get("uid")
	if !ok || val == nil {
		return 0, false
	}

	switch v := val.(type) {
	case uint:
		return v, v > 0
	case uint64:
		return uint(v), v > 0
	case int:
		if v <= 0 {
			return 0, false
		}

		return uint(v), true
	case int64:
		if v <= 0 {
			return 0, false
		}

		return uint(v), true
	case string:
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil || n == 0 {
			return 0, false
		}

		return uint(n), true
	default:
		return 0, false
	}
}
