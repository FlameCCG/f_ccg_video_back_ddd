package middlewares

import (
	"strconv"
	"strings"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	jwts "f_ccg_video_back_ddd/pkg/jwt"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// JwtAuth JWT认证中间件.
func (m *Middlewares) JwtAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		m.authenticateToken(c, strings.TrimSpace(c.GetHeader("Authorization")))
	}
}

// JwtAuthWithQueryToken 允许从指定 query 参数或 Authorization 头读取 token。
func (m *Middlewares) JwtAuthWithQueryToken(queryKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(c.Query(queryKey))
		if token == "" {
			token = strings.TrimSpace(c.GetHeader("Authorization"))
		}

		m.authenticateToken(c, token)
	}
}

// JwtAuthWithQueryTokenPreflight 仅在升级/流式请求的前置校验阶段使用短超时，校验后恢复原请求上下文。
func (m *Middlewares) JwtAuthWithQueryTokenPreflight(queryKey string, timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(c.Query(queryKey))
		if token == "" {
			token = strings.TrimSpace(c.GetHeader("Authorization"))
		}

		originalCtx := c.Request.Context()
		ctx, cancel := ctxutil.WithTimeout(originalCtx, timeout)
		c.Request = c.Request.WithContext(ctx)
		ok := m.authenticateTokenOnly(c, token)

		cancel()

		c.Request = c.Request.WithContext(originalCtx)
		if !ok || shouldSkipNext(c) {
			return
		}

		c.Next()
	}
}

func (m *Middlewares) authenticateToken(c *gin.Context, token string) {
	if !m.authenticateTokenOnly(c, token) {
		return
	}

	if shouldSkipNext(c) {
		return
	}

	c.Next()
}

func (m *Middlewares) authenticateTokenOnly(c *gin.Context, token string) bool {
	if token == "" {
		res.FailWithMsg("请先登录", c)
		c.Abort()

		return false
	}

	token = strings.TrimPrefix(token, "Bearer ")

	uid, username, err := jwts.ParseJwtTokenWithRole(token, m.accessSecret())
	if err != nil {
		m.log().Error("JwtAuth ParseJwtToken failed", zap.Error(err))
		res.FailWithMsg("token无效或已过期", c)
		c.Abort()

		return false
	}

	return m.authenticateResolvedUser(c, uid, username)
}

func (m *Middlewares) authenticateResolvedUser(c *gin.Context, uid, username string) bool {
	if strings.TrimSpace(uid) == "" {
		res.FailWithMsg("请先登录", c)
		c.Abort()

		return false
	}

	c.Set("uid", uid)
	c.Set("username", username)

	if uidInt, err := strconv.ParseUint(uid, 10, 64); err == nil && uidInt > 0 {
		status, err := m.userStatus.CachedStatus(c.Request.Context(), uint(uidInt))
		if err == nil {
			if status == enum.UserStatusPermanentlyBanned {
				res.FailWithMsg("账号已被永久封禁", c)
				c.Abort()

				return false
			}
		}
	}

	return true
}

// accessSecret 读取当前生效的 JWT access 密钥（支持配置热更新）。
func (m *Middlewares) accessSecret() string {
	cfg := m.cfgProvider.Get()
	if cfg == nil || cfg.Jwt == nil {
		return ""
	}

	return cfg.Jwt.AccessSecret
}
