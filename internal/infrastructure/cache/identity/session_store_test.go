package identity

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/config"
	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	jwts "f_ccg_video_back_ddd/pkg/jwt"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestSessions(t *testing.T) (*SessionStore, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	server.SetTime(time.Now())
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewSessionStore(client), server
}

func TestSessionsExpireIndependentlyAndRevokeOnlyTheirUser(t *testing.T) {
	sessions, server := newTestSessions(t)
	for _, session := range []struct {
		name     string
		userID   uint
		duration time.Duration
	}{
		{"web", 7, 2 * time.Hour}, {"phone", 7, time.Hour}, {"other-user", 8, 2 * time.Hour},
	} {
		if err := sessions.Create(t.Context(), session.userID, session.name, "digest", time.Now().Add(session.duration)); err != nil {
			t.Fatal(err)
		}
	}
	server.FastForward(90 * time.Minute)
	if ok, err := sessions.Exists(t.Context(), 7, "phone"); err != nil || ok {
		t.Fatalf("expired phone accepted: %v %v", ok, err)
	}
	if ok, err := sessions.Exists(t.Context(), 7, "web"); err != nil || !ok {
		t.Fatalf("web expired with phone: %v %v", ok, err)
	}
	if err := sessions.RevokeUser(t.Context(), 7); err != nil {
		t.Fatal(err)
	}
	if ok, err := sessions.Exists(t.Context(), 7, "web"); err != nil || ok {
		t.Fatalf("web survived revoke: %v %v", ok, err)
	}
	if ok, err := sessions.Exists(t.Context(), 8, "other-user"); err != nil || !ok {
		t.Fatalf("another user's session revoked: %v %v", ok, err)
	}
	server.FastForward(time.Hour)
	if len(server.Keys()) != 0 {
		t.Fatalf("session index leaked: %v", server.Keys())
	}
}

func TestConcurrentRefreshIsIdempotentAndLateReplayRevokesOnlyThatDevice(t *testing.T) {
	sessions, server := newTestSessions(t)
	expiresAt := time.Now().Add(time.Hour)
	for _, id := range []string{"phone", "web"} {
		if err := sessions.Create(t.Context(), 7, id, "original", expiresAt); err != nil {
			t.Fatal(err)
		}
	}
	var winners atomic.Int32
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			ok, err := sessions.Rotate(t.Context(), 7, "phone", "original", "next", expiresAt)
			if err != nil {
				t.Error(err)
				return
			}
			if ok {
				winners.Add(1)
			}
		})
	}
	wg.Wait()
	if winners.Load() != 2 {
		t.Fatalf("successful refreshes=%d", winners.Load())
	}
	if ok, err := sessions.Exists(t.Context(), 7, "phone"); err != nil || !ok {
		t.Fatalf("retry revoked session: %v %v", ok, err)
	}
	server.SetTime(time.Now().Add(refreshRetryGrace + time.Second))
	if _, err := sessions.Rotate(t.Context(), 7, "phone", "original", "late", expiresAt); !errors.Is(err, app.ErrRefreshTokenReused) {
		t.Fatalf("late replay accepted: %v", err)
	}
	if ok, err := sessions.Exists(t.Context(), 7, "phone"); err != nil || ok {
		t.Fatalf("late replay did not revoke session: %v %v", ok, err)
	}
	if ok, err := sessions.Rotate(t.Context(), 7, "web", "original", "web-next", expiresAt); err != nil || !ok {
		t.Fatalf("phone replay broke web refresh: %v %v", ok, err)
	}
	if ok, err := sessions.Exists(t.Context(), 8, "web"); err != nil || ok {
		t.Fatalf("session usable as another user: %v %v", ok, err)
	}
}

type sessionConfig struct{ conf *config.Config }

func (c sessionConfig) Get() *config.Config { return c.conf }

type sessionUsers struct {
	domain.UserRepository
	user *domain.User
}

func (r sessionUsers) FindByID(context.Context, uint) (*domain.User, error) { return r.user, nil }

func TestRedisSessionsSupportMultiDeviceTokensAndPasswordRevocation(t *testing.T) {
	sessions, server := newTestSessions(t)
	reader := sessionConfig{&config.Config{Jwt: &config.Jwt{AccessExp: 1, RefreshExp: 7, AccessSecret: "access-key", RefreshSecret: "refresh-key"}}}
	tokens := app.NewTokenService(reader, sessions)
	user := domain.RestoreUser(domain.UserSnapshot{ID: 7, Username: "admin", Password: "old-hash", Status: enum.UserStatusActive})
	statuses := app.NewUserStatusService(nil, sessionUsers{user: user}, sessions)
	phone, err := tokens.IssueForUser(t.Context(), user)
	if err != nil {
		t.Fatal(err)
	}
	web, err := tokens.IssueForUser(t.Context(), user)
	if err != nil {
		t.Fatal(err)
	}
	phoneClaims, err := jwts.ParseAccessToken(phone.AccessToken, "access-key")
	if err != nil {
		t.Fatal(err)
	}
	webClaims, err := jwts.ParseAccessToken(web.AccessToken, "access-key")
	if err != nil {
		t.Fatal(err)
	}
	if phoneClaims.SessionID == webClaims.SessionID {
		t.Fatal("devices share a session ID")
	}
	key, _ := sessions.keys(7, phoneClaims.SessionID)
	stored, err := server.Get(key)
	if err != nil || stored != jwts.TokenDigest(phone.RefreshToken) {
		t.Fatalf("refresh digest was not stored: %v", err)
	}
	for _, pair := range []*jwts.JwtToken{phone, web} {
		if _, err := statuses.ValidateAccessToken(t.Context(), pair.AccessToken, "access-key"); err != nil {
			t.Fatal(err)
		}
	}
	nextPhone, err := tokens.Rotate(t.Context(), user, phone.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.Rotate(t.Context(), user, phone.RefreshToken); err != nil {
		t.Fatalf("lost response cannot be retried: %v", err)
	}
	server.SetTime(time.Now().Add(refreshRetryGrace + time.Second))
	if _, err := tokens.Rotate(t.Context(), user, phone.RefreshToken); !errors.Is(err, app.ErrRefreshTokenReused) {
		t.Fatalf("replay accepted: %v", err)
	}
	if _, err := statuses.ValidateAccessToken(t.Context(), nextPhone.AccessToken, "access-key"); err == nil {
		t.Fatal("access survived its session revocation")
	}
	if _, err := statuses.ValidateAccessToken(t.Context(), web.AccessToken, "access-key"); err != nil {
		t.Fatalf("phone replay revoked web: %v", err)
	}
	nextWeb, err := tokens.Rotate(t.Context(), user, web.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	// 模拟密码已提交而 Redis 清理尚未完成：旧版本仍必须立即拒绝。
	if err := user.ResetPassword("new-hash"); err != nil {
		t.Fatal(err)
	}
	if _, err := statuses.ValidateAccessToken(t.Context(), nextWeb.AccessToken, "access-key"); err == nil {
		t.Fatal("old access survived new password")
	}
	if _, err := tokens.Rotate(t.Context(), user, nextWeb.RefreshToken); err == nil {
		t.Fatal("old refresh survived new password")
	}
	if err := sessions.RevokeUser(t.Context(), 7); err != nil {
		t.Fatal(err)
	}
	if len(server.Keys()) != 0 {
		t.Fatalf("old sessions remain after cleanup: %v", server.Keys())
	}
}

func TestRefreshRetriesDoNotExtendConsumedDigestGrace(t *testing.T) {
	sessions, server := newTestSessions(t)
	now := time.Now()
	server.SetTime(now)
	expires := now.Add(time.Hour)
	if err := sessions.Create(t.Context(), 7, "web", "original", expires); err != nil {
		t.Fatal(err)
	}
	for i, next := range []string{"first", "second", "third"} {
		server.SetTime(now.Add(time.Duration(i) * 10 * time.Second))
		if ok, err := sessions.Rotate(t.Context(), 7, "web", "original", next, expires); err != nil || !ok {
			t.Fatalf("retry %d: %v %v", i, ok, err)
		}
	}
	if ok, err := sessions.Rotate(t.Context(), 7, "web", "original", "first", expires); err != nil || !ok {
		t.Fatal("delayed script retry failed", ok, err)
	}
	key, _ := sessions.keys(7, "web")
	if current, err := server.Get(key); err != nil || current != "third" {
		t.Fatal("delayed retry rolled back current digest", current, err)
	}
	// 第一轮的回复更晚才返回时，其令牌也仍可轮换。
	if ok, err := sessions.Rotate(t.Context(), 7, "web", "first", "fourth", expires); err != nil || !ok {
		t.Fatal(ok, err)
	}
	server.SetTime(now.Add(refreshRetryGrace + time.Millisecond))
	if _, err := sessions.Rotate(t.Context(), 7, "web", "original", "late", expires); !errors.Is(err, app.ErrRefreshTokenReused) {
		t.Fatalf("retry extended original grace: %v", err)
	}
	if ok, err := sessions.Rotate(t.Context(), 7, "missing", "old", "new", expires); err != nil || ok {
		t.Fatalf("missing session reported reuse: %v %v", ok, err)
	}
}

type lostReplySessions struct {
	app.SessionStore
	loseNext bool
}

func (s *lostReplySessions) Rotate(ctx context.Context, uid uint, sid, oldHash, newHash string, expires time.Time) (bool, error) {
	ok, err := s.SessionStore.Rotate(ctx, uid, sid, oldHash, newHash, expires)
	if err == nil && ok && s.loseNext {
		s.loseNext = false
		return false, context.DeadlineExceeded
	}
	return ok, err
}

func TestTokenRotationRecoversWhenCommittedReplyIsLost(t *testing.T) {
	sessions, _ := newTestSessions(t)
	reader := sessionConfig{&config.Config{Jwt: &config.Jwt{AccessExp: 1, RefreshExp: 7, AccessSecret: "access", RefreshSecret: "refresh"}}}
	store := &lostReplySessions{SessionStore: sessions, loseNext: true}
	tokens := app.NewTokenService(reader, store)
	user := domain.RestoreUser(domain.UserSnapshot{ID: 7, Password: "hash", Status: enum.UserStatusActive})
	pair, err := tokens.IssueForUser(t.Context(), user)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.Rotate(t.Context(), user, pair.RefreshToken); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	recovered, err := tokens.Rotate(t.Context(), user, pair.RefreshToken)
	if err != nil {
		t.Fatal("client retry revoked the device", err)
	}
	if _, err := tokens.Rotate(t.Context(), user, recovered.RefreshToken); err != nil {
		t.Fatal("recovered token unusable", err)
	}
}

func TestRedisFailureDoesNotIssueOrAcceptTokens(t *testing.T) {
	sessions, server := newTestSessions(t)
	reader := sessionConfig{&config.Config{Jwt: &config.Jwt{AccessExp: 1, RefreshExp: 1, AccessSecret: "key", RefreshSecret: "refresh"}}}
	user := domain.RestoreUser(domain.UserSnapshot{ID: 7, Username: "user", Password: "hash", Status: enum.UserStatusActive})
	tokens := app.NewTokenService(reader, sessions)
	pair, err := tokens.IssueForUser(t.Context(), user)
	if err != nil {
		t.Fatal(err)
	}
	server.SetError("ERR session storage unavailable")
	if _, err := tokens.IssueForUser(t.Context(), user); err == nil {
		t.Fatal("issued tokens without storing session")
	}
	if _, err := tokens.Rotate(t.Context(), user, pair.RefreshToken); err == nil {
		t.Fatal("refreshed without session storage")
	}
	statuses := app.NewUserStatusService(nil, sessionUsers{user: user}, sessions)
	if _, err := statuses.ValidateAccessToken(t.Context(), pair.AccessToken, "key"); err == nil {
		t.Fatal("accepted access while session storage failed")
	}
}

func TestCredentialSnapshotPreservesNewSessionAndIndex(t *testing.T) {
	sessions, server := newTestSessions(t)
	if err := sessions.Create(t.Context(), 7, "old", "old-digest", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	ids, err := sessions.SnapshotUser(t.Context(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if err := sessions.Create(t.Context(), 7, "new", "new-digest", time.Now().Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if ok, err := sessions.Rotate(t.Context(), 7, "old", "old-digest", "rotated", time.Now().Add(time.Hour)); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if err := sessions.RevokeSessions(t.Context(), 7, ids); err != nil {
		t.Fatal(err)
	}
	if ok, err := sessions.Exists(t.Context(), 7, "old"); err != nil || ok {
		t.Fatal("old survived", err)
	}
	if ok, err := sessions.Exists(t.Context(), 7, "new"); err != nil || !ok {
		t.Fatal("new revoked", err)
	}
	ids, err = sessions.SnapshotUser(t.Context(), 7)
	if err != nil || len(ids) != 1 || ids[0] != "new" {
		t.Fatal("lost new session index", ids, err)
	}
	server.FastForward(3 * time.Hour)
	if len(server.Keys()) != 0 {
		t.Fatal("index leaked", server.Keys())
	}
}

func TestRotateStorageFailureDoesNotConsumeRefreshDigest(t *testing.T) {
	sessions, server := newTestSessions(t)
	if err := sessions.Create(t.Context(), 7, "device", "old", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	server.SetError("READONLY temporarily unavailable")
	if _, err := sessions.Rotate(t.Context(), 7, "device", "old", "new", time.Now().Add(time.Hour)); err == nil {
		t.Fatal("storage error accepted")
	}
	server.SetError("")
	key, _ := sessions.keys(7, "device")
	if digest, err := server.Get(key); err != nil || digest != "old" {
		t.Fatal("digest consumed", digest, err)
	}
	if ok, err := sessions.Rotate(t.Context(), 7, "device", "old", "new", time.Now().Add(time.Hour)); err != nil || !ok {
		t.Fatal("retry failed", ok, err)
	}
}
