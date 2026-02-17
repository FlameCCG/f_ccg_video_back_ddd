package middlewares

import (
	"strings"

	jwts "f_ccg_video_back_ddd/pkg/jwt"

	"github.com/gin-gonic/gin"
)

// JwtOptional 解析可选的 JWT，解析成功则把 uid/username 放入上下文，失败或缺失时直接放行.
func (m *Middlewares) JwtOptional() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.Next()
			return
		}

		token = strings.TrimPrefix(token, "Bearer ")
		if uid, username, err := jwts.ParseJwtTokenWithRole(token, m.accessSecret()); err == nil {
			c.Set("uid", uid)
			c.Set("username", username)
		}

		c.Next()
	}
}
