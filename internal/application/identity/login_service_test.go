package identity

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/config"
	"testing"

	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/identity"

	"go.uber.org/zap"
)

const (
	testTester           = "tester"
	testVictimExampleCom = "victim@example.com"
)

type loginConfigReaderStub struct {
	config *config.Config
}

func (s loginConfigReaderStub) Get() *config.Config { return s.config }

type refreshUserRepositoryStub struct {
	domain.UserAccountRepository
	user *domain.User
	err  error
}

func (s refreshUserRepositoryStub) FindByID(context.Context, uint) (*domain.User, error) {
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
	tokens := NewTokenService(reader)

	pair, err := tokens.issueCredentialPair(&domain.User{ID: 7, Username: testTester})
	require.NoError(t, err, "issue token: %v", err)

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

	{
		_, err := service.RefreshToken(context.Background(), pair.RefreshToken)
		require.ErrorIs(t, err, ErrQueryUserFailed, "RefreshToken error = %v, want ErrQueryUserFailed", err)
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
	tokens := NewTokenService(reader)

	pair, err := tokens.issueCredentialPair(&domain.User{ID: 7, Username: testTester})
	require.NoError(t, err, "issue token: %v", err)

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

	{
		_, err := service.RefreshToken(context.Background(), pair.RefreshToken)
		require.ErrorIs(t, err, domain.ErrUserNotFound, "RefreshToken error = %v, want ErrUserNotFound", err)
	}
}
