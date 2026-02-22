package middlewares

import (
	"strings"

	"f_ccg_video_back_ddd/internal/config"
	jwts "f_ccg_video_back_ddd/pkg/jwt"

	"github.com/gin-gonic/gin"
)

// IsLogin 判断请求是否携带有效的 access token，JWT 密钥由参数显式传入。
func IsLogin(c *gin.Context, jwtConf *config.Jwt) bool {
	if c == nil || jwtConf == nil {
		return false
	}

	token := c.GetHeader("Authorization")
	if token == "" {
		return false
	}
	// 去掉 Bearer 前缀
	token = strings.TrimPrefix(token, "Bearer ")
	_, _, err := jwts.ParseJwtToken(token, jwtConf.AccessSecret)

	return err == nil
}
