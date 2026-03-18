package middlewares

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/pkg/authsession"
	"f_ccg_video_back_ddd/pkg/ctxutil"
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

		c.Request = c.Request.WithContext(authsession.Copy(originalCtx, c.Request.Context()))
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

	user, validationErr := m.resolveAccessUser(c, token)
	if validationErr != nil {
		m.log().Warn("JWT credential validation failed", zap.Error(validationErr))
		res.FailWithMsg("token invalid or account unavailable", c)
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
	return m.authenticateResolvedUser(c, strconv.FormatUint(uint64(user.ID), 10), user.Username)
}

type requestAuthentication struct {
	token  string
	secret string
	user   *domain.User
	err    error
}

const requestAuthenticationKey = "middlewares.jwt.authentication"

// resolveAccessUser only reuses validation within this request and for the same
// token and signing key. Long-lived session validators deliberately bypass it.
func (m *Middlewares) resolveAccessUser(c *gin.Context, token string) (*domain.User, error) {
	secret := m.accessSecret()
	if cached, ok := c.Get(requestAuthenticationKey); ok {
		if result, ok := cached.(requestAuthentication); ok && result.token == token && result.secret == secret {
			return result.user, result.err
		}
	}
	if m.userStatus == nil {
		return nil, errors.New("authentication service unavailable")
	}
	user, err := m.userStatus.ValidateAccessToken(c.Request.Context(), token, secret)
	c.Set(requestAuthenticationKey, requestAuthentication{token: token, secret: secret, user: user, err: err})
	return user, err
}

func (m *Middlewares) authenticateResolvedUser(c *gin.Context, uid, username string) bool {
	if strings.TrimSpace(uid) == "" {
		res.FailWithMsg("请先登录", c)
		c.Abort()

		return false
	}

	c.Set("uid", uid)
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
