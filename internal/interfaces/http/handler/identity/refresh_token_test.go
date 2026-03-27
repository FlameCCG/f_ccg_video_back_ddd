package identity

import (
	"context"
	"errors"
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/config"
	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	cache "f_ccg_video_back_ddd/internal/infrastructure/cache/identity"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"strings"
	"testing"
	"time"
)

type refreshFailureUsers struct {
	domain.UserAccountRepository
	user *domain.User
	err  error
}

func (r *refreshFailureUsers) FindByID(context.Context, uint) (*domain.User, error) {
	return r.user, r.err
}

type refreshFailureSessions struct {
	app.SessionStore
	err error
}

func (s refreshFailureSessions) Rotate(ctx context.Context, uid uint, sid, oldHash, newHash string, expires time.Time) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.SessionStore.Rotate(ctx, uid, sid, oldHash, newHash, expires)
}

func TestRefreshTokenViewDistinguishesRejectedCredentialsFromStorageFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name                     string
		dbErr, storeErr          error
		missing, banned, invalid bool
		status                   int
		message                  string
	}{
		{name: "database", dbErr: errors.New("private database address"), status: 503, message: "身份验证暂时不可用"},
		{name: "database timeout", dbErr: context.DeadlineExceeded, status: 504, message: "身份验证超时"},
		{name: "redis", storeErr: errors.New("private Redis address"), status: 503, message: "身份验证暂时不可用"},
		{name: "redis timeout", storeErr: context.DeadlineExceeded, status: 504, message: "身份验证超时"},
		{name: "invalid", invalid: true, status: 200, message: "refreshToken无效"},
		{name: "missing user", missing: true, status: 200, message: "用户不存在"},
		{name: "banned", banned: true, status: 200, message: "account is banned"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: server.Addr()})
			t.Cleanup(func() {
				if err := client.Close(); err != nil {
					t.Error(err)
				}
			})
			store := cache.NewSessionStore(client)
			provider := infraconfig.NewProvider(&config.Config{Jwt: &config.Jwt{AccessExp: 1, RefreshExp: 1, AccessSecret: "access", RefreshSecret: "refresh"}})
			tokens := app.NewTokenService(provider, refreshFailureSessions{SessionStore: store, err: tc.storeErr})
			user := domain.RestoreUser(domain.UserSnapshot{ID: 7, Password: "hash", Status: enum.UserStatusActive})
			pair, err := tokens.IssueForUser(t.Context(), user)
			if err != nil {
				t.Fatal(err)
			}
			repo := &refreshFailureUsers{user: user, err: tc.dbErr}
			if tc.missing {
				repo.user = nil
			}
			if tc.banned {
				snapshot := user.Snapshot()
				snapshot.Status = enum.UserStatusBanned
				repo.user = domain.RestoreUser(snapshot)
			}
			handler := &Handler{logins: app.NewLoginService(provider, repo, nil, tokens, nil, nil, nil, zap.NewNop())}
			token := pair.RefreshToken
			if tc.invalid {
				token = "bad-signature"
			}
			rec, ctx := newTestContext("POST", "/v1/common/user/login/refresh")
			ctx.Set("request", RefreshTokenReq{RefreshToken: token})
			handler.RefreshTokenView(ctx)
			if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.message) || strings.Contains(rec.Body.String(), "private") {
				t.Fatalf("response=%d %s", rec.Code, rec.Body.String())
			}
			if tc.dbErr != nil || tc.storeErr != nil {
				if _, err := app.NewTokenService(provider, store).Rotate(t.Context(), user, pair.RefreshToken); err != nil {
					t.Fatal("unconsumed token cannot recover", err)
				}
			}
		})
	}
}
