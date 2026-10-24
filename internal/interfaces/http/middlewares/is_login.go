package middlewares

import (
	"github.com/gin-gonic/gin"
	"strings"
)

// IsLogin checks the current revocable account state, not just the JWT signature.
func (m *Middlewares) IsLogin(c *gin.Context) bool {
	if c == nil || m == nil || m.userStatus == nil {
		return false
	}
	token := strings.TrimPrefix(strings.TrimSpace(c.GetHeader("Authorization")), "Bearer ")
	user, err := m.userStatus.ValidateAccessToken(c.Request.Context(), token, m.accessSecret())
	return err == nil && !user.MustChangePassword
}
