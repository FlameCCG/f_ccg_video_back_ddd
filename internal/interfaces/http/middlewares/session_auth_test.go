package middlewares

import (
	"context"
	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/config"
	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/pkg/authsession"
	jwts "f_ccg_video_back_ddd/pkg/jwt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type middlewareCredentialUsers struct {
	domain.UserRepository
	user    *domain.User
	queries int
}

func (r *middlewareCredentialUsers) FindByID(context.Context, uint) (*domain.User, error) {
	r.queries++
	u := *r.user
	return &u, nil
}

func TestAuthenticationReusesRequestValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &domain.User{ID: 7, Username: "current-name", Password: "hash", Status: enum.UserStatusActive}
	users := &middlewareCredentialUsers{user: user}
	provider := infraconfig.NewProvider(&config.Config{Jwt: &config.Jwt{AccessSecret: "key"}})
	mw := New(provider, nil, identityapp.NewUserStatusService(nil, users), nil, nil, nil, nil, nil, zap.NewNop())
	pair, err := jwts.CreateJwtTokenWithPassword("7", "old-name", user.Password, false, time.Hour, "key", time.Hour, "refresh")
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	engine.GET("/private", mw.JwtOptional(), mw.JwtAuth(), mw.JwtAuth(), func(c *gin.Context) {
		if c.GetString("uid") != "7" || c.GetString("username") != user.Username {
			t.Fatal("validated identity missing")
		}
		c.String(200, "allowed")
	})
	request := func() string {
		req := httptest.NewRequest(http.MethodGet, "/private", nil)
		req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec.Body.String()
	}
	if body := request(); body != "allowed" || users.queries != 1 {
		t.Fatalf("body = %q, queries = %d; want allowed and one query", body, users.queries)
	}
	user.Password = "changed"
	if body := request(); body == "allowed" || users.queries != 2 {
		t.Fatalf("revoked credentials reused across requests: body=%q queries=%d", body, users.queries)
	}
}
func TestInitialPasswordGateAndCredentialRevocation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &domain.User{ID: 7, Username: "admin", Password: "original-hash", MustChangePassword: true, Status: enum.UserStatusActive}
	users := &middlewareCredentialUsers{user: user}
	statuses := identityapp.NewUserStatusService(nil, users)
	provider := infraconfig.NewProvider(&config.Config{Jwt: &config.Jwt{AccessSecret: "access-key"}})
	mw := New(provider, nil, statuses, nil, nil, nil, nil, nil, zap.NewNop())
	engine := gin.New()
	engine.GET("/private", mw.JwtAuth(), func(c *gin.Context) { c.String(200, "protected action") })
	engine.PUT("/v1/common/user/password/change", mw.JwtAuth(), func(c *gin.Context) { c.String(200, "password change allowed") })
	pair, err := jwts.CreateJwtTokenWithPassword("7", "admin", user.Password, true, time.Hour, "access-key", time.Hour, "refresh-key")
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, token string) string {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		engine.ServeHTTP(rec, req)
		return rec.Body.String()
	}
	if body := request(http.MethodGet, "/private", pair.AccessToken); strings.Contains(body, "protected action") {
		t.Fatal("initial password accessed protected route")
	}
	if body := request(http.MethodPut, "/v1/common/user/password/change", pair.AccessToken); body != "password change allowed" {
		t.Fatalf("change blocked: %s", body)
	}
	user.Password = "new-hash"
	user.MustChangePassword = false
	if body := request(http.MethodGet, "/private", pair.AccessToken); strings.Contains(body, "protected action") {
		t.Fatal("revoked token accessed protected route")
	}
}
func TestWebsocketPreflightPreservesRevocationValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &domain.User{ID: 7, Password: "hash", Status: enum.UserStatusActive}
	provider := infraconfig.NewProvider(&config.Config{Jwt: &config.Jwt{AccessSecret: "key"}})
	statuses := identityapp.NewUserStatusService(nil, &middlewareCredentialUsers{user: user})
	mw := New(provider, nil, statuses, nil, nil, nil, nil, nil, zap.NewNop())
	pair, err := jwts.CreateJwtTokenWithPassword("7", "user", user.Password, false, time.Hour, "key", time.Hour, "refresh")
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	engine.GET("/ws", mw.JwtAuthWithQueryTokenPreflight("token", time.Second), func(c *gin.Context) {
		if err := authsession.Validate(c.Request.Context()); err != nil {
			t.Fatal(err)
		}
		user.Password = "changed"
		if err := authsession.Validate(c.Request.Context()); err == nil {
			t.Fatal("open socket survived password change")
		}
		c.String(200, "checked")
	})
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Authorization", pair.AccessToken)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Body.String() != "checked" {
		t.Fatalf("preflight failed: %s", rec.Body.String())
	}
}
