package middlewares

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/config"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/pkg/authsession"
	jwts "f_ccg_video_back_ddd/pkg/jwt"

	identitycache "f_ccg_video_back_ddd/internal/infrastructure/cache/identity"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type middlewareCredentialUsers struct {
	identitydomain.UserRepository
	user    *identitydomain.User
	queries int
	err     error
}

func middlewareSessions(t *testing.T) identityapp.SessionStore {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	sessions := identitycache.NewSessionStore(client)
	if err := sessions.Create(t.Context(), 7, "test-session", "digest", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	return sessions
}

func (r *middlewareCredentialUsers) FindByID(context.Context, uint) (*identitydomain.User, error) {
	r.queries++
	if r.err != nil {
		return nil, r.err
	}
	u := *r.user
	return &u, nil
}

func TestAuthenticationReusesRequestValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 7, Username: "current-name", Password: "hash", Status: enum.UserStatusActive})
	users := &middlewareCredentialUsers{user: user}
	provider := infraconfig.NewProvider(&config.Config{Jwt: &config.Jwt{AccessSecret: "key"}})
	mw := New(provider, nil, identityapp.NewUserStatusService(nil, users, middlewareSessions(t)), nil, nil, nil, nil, nil, zap.NewNop())
	pair, err := jwts.CreateJwtTokenWithPassword("7", "old-name", user.Password(), "test-session", time.Hour, "key", time.Hour, "refresh")
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	engine.GET("/private", mw.JwtOptional(), mw.JwtAuth(), mw.JwtAuth(), func(c *gin.Context) {
		if c.GetString("uid") != "7" || c.GetString("username") != user.Username() {
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
	snapshot := user.Snapshot()
	snapshot.Password = "changed"
	*user = *identitydomain.RestoreUser(snapshot)
	if body := request(); body == "allowed" || users.queries != 2 {
		t.Fatalf("revoked credentials reused across requests: body=%q queries=%d", body, users.queries)
	}
}
func TestAdministratorCanUseInitialPasswordAndPasswordChangeRevokesToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 7, Username: "admin", Password: "original-hash", Status: enum.UserStatusActive})
	users := &middlewareCredentialUsers{user: user}
	statuses := identityapp.NewUserStatusService(nil, users, middlewareSessions(t))
	provider := infraconfig.NewProvider(&config.Config{Jwt: &config.Jwt{AccessSecret: "access-key"}})
	mw := New(provider, nil, statuses, nil, nil, nil, nil, nil, zap.NewNop())
	engine := gin.New()
	engine.GET("/private", mw.JwtAuth(), func(c *gin.Context) { c.String(200, "protected action") })
	engine.PUT("/v1/common/user/password/change", mw.JwtAuth(), func(c *gin.Context) { c.String(200, "password change allowed") })
	pair, err := jwts.CreateJwtTokenWithPassword("7", "admin", user.Password(), "test-session", time.Hour, "access-key", time.Hour, "refresh-key")
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
	if body := request(http.MethodGet, "/private", pair.AccessToken); body != "protected action" {
		t.Fatalf("initial password unnecessarily blocked: %s", body)
	}
	if body := request(http.MethodPut, "/v1/common/user/password/change", pair.AccessToken); body != "password change allowed" {
		t.Fatalf("change blocked: %s", body)
	}
	snapshot := user.Snapshot()
	snapshot.Password = "new-hash"
	*user = *identitydomain.RestoreUser(snapshot)
	if body := request(http.MethodGet, "/private", pair.AccessToken); body == "protected action" {
		t.Fatal("revoked token accessed protected route")
	}
}
func TestWebsocketPreflightPreservesRevocationValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 7, Password: "hash", Status: enum.UserStatusActive})
	provider := infraconfig.NewProvider(&config.Config{Jwt: &config.Jwt{AccessSecret: "key"}})
	statuses := identityapp.NewUserStatusService(nil, &middlewareCredentialUsers{user: user}, middlewareSessions(t))
	mw := New(provider, nil, statuses, nil, nil, nil, nil, nil, zap.NewNop())
	pair, err := jwts.CreateJwtTokenWithPassword("7", "user", user.Password(), "test-session", time.Hour, "key", time.Hour, "refresh")
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	engine.GET("/ws", mw.JwtAuthWithQueryTokenPreflight("token", time.Second), func(c *gin.Context) {
		if err := authsession.Validate(c.Request.Context()); err != nil {
			t.Fatal(err)
		}
		snapshot := user.Snapshot()
		snapshot.Password = "changed"
		*user = *identitydomain.RestoreUser(snapshot)
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

func TestHTTPAuthenticationSeparatesStorageFailureFromInvalidSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name    string
		err     error
		invalid bool
		status  int
		message string
	}{
		{name: "database", err: errors.New("private database credentials"), status: 503, message: "身份验证暂时不可用"},
		{name: "timeout", err: context.DeadlineExceeded, status: 504, message: "身份验证超时"},
		{name: "invalid", invalid: true, status: 200, message: "token invalid or account unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 7, Password: "hash", Status: enum.UserStatusActive})
			users := &middlewareCredentialUsers{user: user, err: tc.err}
			provider := infraconfig.NewProvider(&config.Config{Jwt: &config.Jwt{AccessSecret: "key"}})
			mw := New(provider, nil, identityapp.NewUserStatusService(nil, users, middlewareSessions(t)), nil, nil, nil, nil, nil, zap.NewNop())
			pair, err := jwts.CreateJwtTokenWithPassword("7", "tester", "hash", "test-session", time.Hour, "key", time.Hour, "refresh")
			if err != nil {
				t.Fatal(err)
			}
			token := pair.AccessToken
			if tc.invalid {
				token = "bad-signature"
			}
			engine := gin.New()
			engine.GET("/private", mw.JwtAuth(), func(c *gin.Context) { t.Error("invalid authentication reached handler") })
			req := httptest.NewRequest(http.MethodGet, "/private", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.message) || strings.Contains(rec.Body.String(), "private database") {
				t.Fatalf("response=%d %s", rec.Code, rec.Body.String())
			}
		})
	}
}
