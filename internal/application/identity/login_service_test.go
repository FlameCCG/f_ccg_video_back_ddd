package identity

import (
	"context"
	"errors"
	"testing"

	"f_ccg_video_back_ddd/internal/config"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"

	"go.uber.org/zap"
)

type loginConfigReaderStub struct {
	config *config.Config
}

func (s loginConfigReaderStub) Get() *config.Config { return s.config }

type refreshUserRepositoryStub struct {
	identitydomain.UserAccountRepository
	user *identitydomain.User
	err  error
}

func (s refreshUserRepositoryStub) FindByID(context.Context, uint) (*identitydomain.User, error) {
	return s.user, s.err
}

func TestLoginServiceRefreshTokenDistinguishesQueryFailure(t *testing.T) {
	t.Parallel()

	reader := loginConfigReaderStub{config: &config.Config{Jwt: &config.Jwt{
		AccessExp:     1,
		RefreshExp:    1,
		AccessSecret:  "test-access-secret",
		RefreshSecret: "test-refresh-secret",
	}}}
	tokens := NewTokenService(reader, nil)
	pair, err := tokens.issueCredentialPair(identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 7, Username: "tester"}), "test-session")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	dbErr := errors.New("database unavailable")
	service := NewLoginService(
		reader,
		refreshUserRepositoryStub{err: dbErr},
		nil,
		tokens,
		nil,
		nil,
		nil,
		zap.NewNop(),
	)

	if _, err := service.RefreshToken(context.Background(), pair.RefreshToken); !errors.Is(err, ErrQueryUserFailed) {
		t.Fatalf("RefreshToken error = %v, want ErrQueryUserFailed", err)
	}
}

func TestLoginServiceRefreshTokenReportsMissingUser(t *testing.T) {
	t.Parallel()

	reader := loginConfigReaderStub{config: &config.Config{Jwt: &config.Jwt{
		AccessExp:     1,
		RefreshExp:    1,
		AccessSecret:  "test-access-secret",
		RefreshSecret: "test-refresh-secret",
	}}}
	tokens := NewTokenService(reader, nil)
	pair, err := tokens.issueCredentialPair(identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 7, Username: "tester"}), "test-session")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	service := NewLoginService(
		reader,
		refreshUserRepositoryStub{},
		nil,
		tokens,
		nil,
		nil,
		nil,
		zap.NewNop(),
	)

	if _, err := service.RefreshToken(context.Background(), pair.RefreshToken); !errors.Is(err, identitydomain.ErrUserNotFound) {
		t.Fatalf("RefreshToken error = %v, want ErrUserNotFound", err)
	}
}

func TestLoginSessionFailuresPreserveDependencyCause(t *testing.T) {
	for _, cause := range []error{errors.New("redis unavailable"), context.DeadlineExceeded} {
		t.Run(cause.Error(), func(t *testing.T) {
			tokens, _ := newSecurityTokens()
			tokens.sessions.(*securitySessionStore).err = cause
			user := identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 7, Username: "tester", Status: 1})
			password := &LoginService{tokens: tokens, logger: zap.NewNop()}
			oauth := &OAuthLoginService{tokens: tokens, roles: &userInfoRoleQuery{}, logger: zap.NewNop()}
			for _, login := range []func() error{
				func() error {
					_, err := password.finishLogin(t.Context(), user, PwdLoginInput{}, loginTypeAdminPwd)
					return err
				},
				func() error { _, err := oauth.completeLogin(t.Context(), user, OAuthLoginInput{}); return err },
			} {
				err := login()
				if !errors.Is(err, ErrSessionIssueFailed) || !errors.Is(err, cause) || errors.Is(err, ErrLoginFailed) {
					t.Fatalf("lost dependency classification: %v", err)
				}
			}
		})
	}
}
