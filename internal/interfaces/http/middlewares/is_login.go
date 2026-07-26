package middlewares

import (
	"strings"

	"f_ccg_video_back_ddd/internal/infrastructure/config"
	jwts "f_ccg_video_back_ddd/pkg/jwt"

	"github.com/gin-gonic/gin"
)

// IsLogin 判断请求是否携带有效的 access token。
//
// 迁移自原 service/user_service/enter.go 的 UserService.isLogin / IsLogin：
// 该函数读取 HTTP 请求头，属于接口层关注点，且原项目中无任何调用方（死代码）。
// 为不丢弃原有能力而完整保留，JWT 密钥改为显式传入（见 report.md FIND-006）。
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
