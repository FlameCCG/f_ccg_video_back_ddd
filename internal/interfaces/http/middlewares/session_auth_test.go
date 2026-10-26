package middlewares

import (
	"context"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/authsession"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	identityapp "f_ccg_video_back_ddd/internal/application/identity"

	domain "f_ccg_video_back_ddd/internal/domain/identity"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	jwts "f_ccg_video_back_ddd/pkg/jwt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type middlewareCredentialUsers struct {
	domain.UserRepository
	user *domain.User
}

func (r *middlewareCredentialUsers) FindByID(context.Context, uint) (*domain.User, error) {
	u := *r.user
	return &u, nil
}

func TestInitialPasswordGateAndCredentialRevocation(t *testing.T) {
	t.Parallel()

	user := &domain.User{ID: 7, Username: "admin", Password: "original-hash", MustChangePassword: true, Status: enum.UserStatusActive}
	users := &middlewareCredentialUsers{user: user}
	statuses := identityapp.NewUserStatusService(nil, users)
	provider := infraconfig.NewProvider(&config.Config{Jwt: &config.Jwt{AccessSecret: "access-key"}})
	mw := New(provider, nil, statuses, nil, nil, nil, nil, nil, zap.NewNop())
	engine := gin.New()
	engine.GET("/private", mw.JwtAuth(), func(c *gin.Context) { c.String(200, "protected action") })
	engine.PUT("/v1/common/user/password/change", mw.JwtAuth(), func(c *gin.Context) { c.String(200, "password change allowed") })

	pair, err := jwts.CreateJwtTokenWithPassword("7", "admin", user.Password, true, time.Hour, "access-key", time.Hour, "refresh-key")
	require.NoError(t, err)

	request := func(method, path, token string) string {
		t.Helper()

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), method, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		engine.ServeHTTP(rec, req)

		return rec.Body.String()
	}
	{
		body := request(http.MethodGet, "/private", pair.AccessToken)
		require.NotContains(t, body, "protected action", "initial password accessed protected route")
	}

	{
		body := request(http.MethodPut, "/v1/common/user/password/change", pair.AccessToken)
		require.Equal(t, "password change allowed", body, "change blocked: %s", body)
	}

	user.Password = "new-hash"
	user.MustChangePassword = false

	{
		body := request(http.MethodGet, "/private", pair.AccessToken)
		require.NotContains(t, body, "protected action", "revoked token accessed protected route")
	}
}

func TestWebsocketPreflightPreservesRevocationValidator(t *testing.T) {
	t.Parallel()

	user := &domain.User{ID: 7, Password: "hash", Status: enum.UserStatusActive}
	provider := infraconfig.NewProvider(&config.Config{Jwt: &config.Jwt{AccessSecret: "key"}})
	statuses := identityapp.NewUserStatusService(nil, &middlewareCredentialUsers{user: user})
	mw := New(provider, nil, statuses, nil, nil, nil, nil, nil, zap.NewNop())

	pair, err := jwts.CreateJwtTokenWithPassword("7", "user", user.Password, false, time.Hour, "key", time.Hour, "refresh")
	require.NoError(t, err)

	engine := gin.New()
	engine.GET("/ws", mw.JwtAuthWithQueryTokenPreflight("token", time.Second), func(c *gin.Context) {
		{
			err := authsession.Validate(c.Request.Context())
			require.NoError(t, err)
		}

		user.Password = "changed"

		{
			err := authsession.Validate(c.Request.Context())
			require.Error(t, err, "open socket survived password change")
		}

		c.String(200, "checked")
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ws", nil)
	req.Header.Set("Authorization", pair.AccessToken)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	require.Equal(t, "checked", rec.Body.String(), "preflight failed: %s", rec.Body.String())
}
