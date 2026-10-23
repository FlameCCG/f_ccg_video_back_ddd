package middlewares

import (
	"strings"

	jwts "f_ccg_video_back_ddd/pkg/jwt"

	"github.com/gin-gonic/gin"
)

// JwtOptional 解析可选的 JWT，解析成功则把 uid/username 放入上下文，失败或缺失时直接放行.
//
// 这是公开接口的身份增强，令牌无效会按匿名继续，不提供“必须登录”的保证。
// 需要身份的写接口应使用强制认证链，并由应用用例继续检查资源归属。
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
