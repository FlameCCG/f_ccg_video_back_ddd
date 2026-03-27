package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/config"
	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	identitycache "f_ccg_video_back_ddd/internal/infrastructure/cache/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/pkg/encrypt"
	jwts "f_ccg_video_back_ddd/pkg/jwt"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type accountRoleFacts struct {
	app.UserRoleQuery
	owned               bool
	transactionObserved bool
}

type failedSessionCleanup struct{ app.SessionStore }

func (failedSessionCleanup) RevokeSessions(context.Context, uint, []string) error {
	return errors.New("redis cleanup failed")
}

func TestPasswordResetRevokesTokensEvenWhenRedisCleanupFails(t *testing.T) {
	db := openUserProfileTestDB(t)
	repo := NewUserRepository(db, nil)
	sessions := accountSessions(t)
	reader := credentialTestConfig{&config.Config{Jwt: &config.Jwt{AccessExp: 1, RefreshExp: 7, AccessSecret: "access", RefreshSecret: "refresh"}}}
	tokens := app.NewTokenService(reader, sessions)
	user, err := repo.FindByID(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	phone, err := tokens.IssueForUser(t.Context(), user)
	if err != nil {
		t.Fatal(err)
	}
	web, err := tokens.IssueForUser(t.Context(), user)
	if err != nil {
		t.Fatal(err)
	}
	service := app.NewAccountService(gormx.NewTxManager(db), repo, nil, failedSessionCleanup{sessions}, nil)
	if err := service.ResetPassword(t.Context(), app.ResetPasswordInput{Email: "joined-user@example.com", NewPassword: "replacement"}); err != nil {
		t.Fatalf("committed password change incorrectly reported failure: %v", err)
	}
	user, err = repo.FindByID(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	statuses := app.NewUserStatusService(nil, repo, sessions)
	for _, pair := range []*jwts.JwtToken{phone, web} {
		if _, err := statuses.ValidateAccessToken(t.Context(), pair.AccessToken, "access"); err == nil {
			t.Fatal("old access survived failed Redis cleanup")
		}
		if _, err := tokens.Rotate(t.Context(), user, pair.RefreshToken); err == nil {
			t.Fatal("old refresh survived failed Redis cleanup")
		}
	}
}

type credentialTestConfig struct{ conf *config.Config }

func (c credentialTestConfig) Get() *config.Config { return c.conf }

func accountSessions(t *testing.T) app.SessionStore {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	sessions := identitycache.NewSessionStore(client)
	for _, id := range []string{"phone", "web"} {
		if err := sessions.Create(t.Context(), 1, id, "digest", time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	return sessions
}

func (f *accountRoleFacts) HasRole(ctx context.Context, _, _ uint) (bool, error) {
	f.transactionObserved = gormx.TxFrom(ctx) != nil
	return f.owned, nil
}

type abortAccountTransaction struct{ inner tx.Manager }

func (m abortAccountTransaction) Do(ctx context.Context, fn func(context.Context) error) error {
	return m.inner.Do(ctx, func(txCtx context.Context) error {
		if err := fn(txCtx); err != nil {
			return err
		}
		return errors.New("transaction cannot commit")
	})
}

func TestAccountActionsSaveOnlyChangedFieldsAndPropagateTransaction(t *testing.T) {
	db := openUserProfileTestDB(t)
	repo := NewUserRepository(db, nil)
	roles := &accountRoleFacts{}
	sessions := accountSessions(t)
	service := app.NewAccountService(gormx.NewTxManager(db), repo, roles, sessions, nil)
	if _, err := service.BindEmail(t.Context(), app.BindEmailInput{UserID: 1, Email: "no-conf@example.com"}); !errors.Is(err, domain.ErrEmailOccupied) {
		t.Fatal(err)
	}
	if _, err := service.BindEmail(t.Context(), app.BindEmailInput{UserID: 1, Email: "new@example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := service.SwitchRole(t.Context(), 1, 3); !errors.Is(err, domain.ErrRoleNotOwned) {
		t.Fatal(err)
	}
	user, err := repo.FindByID(t.Context(), 1)
	if err != nil || user.CurrentRoleID() != 2 {
		t.Fatal("unowned role changed current selection", err)
	}
	roles.owned = true
	if err := service.SwitchRole(t.Context(), 1, 3); err != nil {
		t.Fatal(err)
	}
	if !roles.transactionObserved {
		t.Fatal("cross-context role query lost transaction context")
	}
	if err := service.ResetPassword(t.Context(), app.ResetPasswordInput{Email: "new@example.com", NewPassword: "replacement"}); err != nil {
		t.Fatal(err)
	}
	user, err = repo.FindByID(t.Context(), 1)
	if err != nil || !encrypt.ValidatePasswordHash("replacement", user.Password()) {
		t.Fatal("reset did not clear all credential state", err)
	}
	if user.CoinCount() != 4.5 || user.CurrentRoleID() != 3 || user.Email() != "new@example.com" || user.Username() != "joined-user" || user.Avatar() != "avatar.png" {
		t.Fatal("field save overwrote unrelated state", user.Snapshot())
	}
	for _, id := range []string{"phone", "web"} {
		if ok, err := sessions.Exists(t.Context(), 1, id); err != nil || ok {
			t.Fatalf("password reset left session %s: %v %v", id, ok, err)
		}
	}
}

func TestAccountCredentialSaveRollsBackAndPreservesFailureContract(t *testing.T) {
	db := openUserProfileTestDB(t)
	repo := NewUserRepository(db, nil)
	sessions := accountSessions(t)
	service := app.NewAccountService(abortAccountTransaction{gormx.NewTxManager(db)}, repo, nil, sessions, nil)
	if err := service.ResetPassword(t.Context(), app.ResetPasswordInput{Email: "joined-user@example.com", NewPassword: "replacement"}); !errors.Is(err, app.ErrResetPasswordFailed) {
		t.Fatal("transaction failure changed public error", err)
	}
	user, err := repo.FindByID(t.Context(), 1)
	if err != nil || user.Password() != "password-hash" {
		t.Fatal("failed transaction leaked credential changes", err)
	}
	for _, id := range []string{"phone", "web"} {
		if ok, err := sessions.Exists(t.Context(), 1, id); err != nil || !ok {
			t.Fatalf("failed password reset revoked session %s: %v %v", id, ok, err)
		}
	}
}

type afterCredentialCommit struct {
	tx.Manager
	after func()
}

func (m afterCredentialCommit) Do(ctx context.Context, fn func(context.Context) error) error {
	if err := m.Manager.Do(ctx, fn); err != nil {
		return err
	}
	m.after()
	return nil
}

func TestPasswordCleanupPreservesSessionCreatedAfterCommit(t *testing.T) {
	db := openUserProfileTestDB(t)
	repo := NewUserRepository(db, nil)
	sessions := accountSessions(t)
	committed := false
	manager := afterCredentialCommit{Manager: gormx.NewTxManager(db), after: func() {
		committed = true
		if err := sessions.Create(t.Context(), 1, "new-password-login", "new", time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}}
	service := app.NewAccountService(manager, repo, nil, sessions, nil)
	if err := service.ResetPassword(t.Context(), app.ResetPasswordInput{Email: "joined-user@example.com", NewPassword: "replacement"}); err != nil {
		t.Fatal(err)
	}
	if !committed {
		t.Fatal("commit hook not called")
	}
	if ok, err := sessions.Exists(t.Context(), 1, "new-password-login"); err != nil || !ok {
		t.Fatal("new session revoked", err)
	}
	for _, id := range []string{"phone", "web"} {
		if ok, err := sessions.Exists(t.Context(), 1, id); err != nil || ok {
			t.Fatal("old session survived", id, err)
		}
	}
}
