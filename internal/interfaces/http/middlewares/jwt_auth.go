package middlewares

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/authsession"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"f_ccg_video_back_ddd/pkg/logmeta"
	"f_ccg_video_back_ddd/pkg/res"
	"strings"
	"time"

	jwts "f_ccg_video_back_ddd/pkg/jwt"

	"github.com/gin-gonic/gin"
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
//
// 认证查询可以有短预算，但将其直接沿用到 WebSocket/SSE 会使长连接在短时间后被取消。
// 恢复原 context 后，长连接仍需响应客户端断开和其自身生命周期取消。
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

		c.Request = c.Request.WithContext(logmeta.Copy(authsession.Copy(originalCtx, c.Request.Context()), c.Request.Context()))
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
		res.FailWithCause(apperr.Wrap("auth.token", nil, err, apperr.Rejected), "token无效或已过期", c)
		c.Abort()

		return false
	}

	if m.userStatus == nil {
		res.RecordFailure(apperr.Wrap("auth.user_status", nil, errors.New("user status unavailable"), apperr.Internal), c)
		c.Abort()

		return false
	}

	user, validationErr := m.userStatus.ValidateAccessToken(c.Request.Context(), token, m.accessSecret())
	if validationErr != nil {
		res.FailWithCause(validationErr, "token invalid or account unavailable", c)
		c.Abort()

		return false
	}

	if user.MustChangePassword && c.FullPath() != "/v1/common/user/password/change" {
		res.FailWithMsg("please change your initial password", c)
		c.Abort()

		return false
	}

	c.Request = c.Request.WithContext(authsession.WithValidator(c.Request.Context(), func(ctx context.Context) error {
		current, err := m.userStatus.ValidateAccessToken(ctx, token, m.accessSecret())
		if err != nil {
			return err
		}

		if current.MustChangePassword {
			return errors.New("initial password change required")
		}

		return nil
	}))

	return m.authenticateResolvedUser(c, uid, username)
}

func (m *Middlewares) authenticateResolvedUser(c *gin.Context, uid, username string) bool {
	if strings.TrimSpace(uid) == "" {
		res.FailWithMsg("请先登录", c)
		c.Abort()

		return false
	}

	c.Set("uid", uid)
	values := logmeta.From(c.Request.Context())
	values.UserID = uid
	c.Request = c.Request.WithContext(logmeta.With(c.Request.Context(), values))
	c.Set("username", username)

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
