package middlewares

import "github.com/gin-gonic/gin"

const skipNextKey = "skip_middleware_next"

// AuthAll 方法或函数.
func (m *Middlewares) AuthAll() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(skipNextKey, true)
		// 先执行 JWT 认证
		m.JwtAuth()(c)

		if c.IsAborted() {
			return
		}
		// 再执行 Casbin 权限验证
		m.CasbinAuth()(c)

		if c.IsAborted() {
			return
		}

		c.Set(skipNextKey, false)
		c.Next()
	}
}

// AuthAllWithQueryToken is AuthAll but allows JWT from a query parameter (EventSource / SSE).
func (m *Middlewares) AuthAllWithQueryToken(queryKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(skipNextKey, true)
		m.JwtAuthWithQueryToken(queryKey)(c)

		if c.IsAborted() {
			return
		}

		m.CasbinAuth()(c)

		if c.IsAborted() {
			return
		}

		c.Set(skipNextKey, false)
		c.Next()
	}
}

func shouldSkipNext(c *gin.Context) bool {
	if v, ok := c.Get(skipNextKey); ok {
		if val, ok := v.(bool); ok {
			return val
		}
	}

	return false
}
