package identity

import (
	"context"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/identity"

	"go.uber.org/zap"
)

type securityAccountRepo struct {
	domain.UserAccountRepository
	mu   sync.Mutex
	user *domain.User
}

func (r *securityAccountRepo) FindByID(context.Context, uint) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	u := *r.user

	return &u, nil
}

func (r *securityAccountRepo) FindByOpenIDAndSource(context.Context, string, string) (*domain.User, error) {
	return nil, nil
}

func (r *securityAccountRepo) FindByEmail(context.Context, string) (*domain.User, error) {
	return r.user, nil
}

func (r *securityAccountRepo) SetRefreshToken(_ context.Context, id uint, pwd, digest string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.user.ID != id || r.user.Password != pwd {
		return false, nil
	}

	r.user.RefreshTokenHash = digest

	return true, nil
}

func (r *securityAccountRepo) RotateRefreshToken(_ context.Context, id uint, pwd, old, next string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.user.ID != id || r.user.Password != pwd || r.user.RefreshTokenHash != old {
		return false, nil
	}

	r.user.RefreshTokenHash = next

	return true, nil
}

type securityStatusRepo struct {
	domain.UserRepository
	account *securityAccountRepo
}

func (r securityStatusRepo) FindByID(ctx context.Context, id uint) (*domain.User, error) {
	return r.account.FindByID(ctx, id)
}

func newSecurityTokens() (*TokenService, loginConfigReaderStub) {
	reader := loginConfigReaderStub{config: &config.Config{Jwt: &config.Jwt{AccessExp: 1, RefreshExp: 1, AccessSecret: "access-key", RefreshSecret: "refresh-key"}}}
	return NewTokenService(reader), reader
}

func TestOAuthDoesNotLinkPreexistingEmailAccount(t *testing.T) {
	t.Parallel()

	repo := &securityAccountRepo{user: &domain.User{ID: 9, Email: testVictimExampleCom}}
	service := &OAuthLoginService{users: repo}

	user, err := service.findOrCreateUser(t.Context(), domain.OAuthProfile{RegisterSource: OAuthProviderGitHub, OpenID: "attacker", Email: testVictimExampleCom})
	require.ErrorIs(t, err, ErrOAuthAccountBindingRequired, "account linked: user=%v error=%v", user, err)
	require.Nil(t, user, "account linked: user=%v error=%v", user, err)
}

func TestPasswordChangeRevokesAccessAndRefreshAcrossServices(t *testing.T) {
	t.Parallel()

	tokens, _ := newSecurityTokens()
	repo := &securityAccountRepo{user: &domain.User{ID: 7, Username: testTester, Password: "salted-old-hash", Status: enum.UserStatusActive}} // #nosec G101 -- Deliberately fake credentials used only by this isolated test fixture.

	pair, err := tokens.IssueForUser(t.Context(), repo.user, repo)
	require.NoError(t, err)

	status := NewUserStatusService(nil, securityStatusRepo{account: repo})
	{
		_, err := status.ValidateAccessToken(t.Context(), pair.AccessToken, "access-key")
		require.NoError(t, err)
	}

	repo.user.Password = "salted-new-hash"

	{
		_, err := status.ValidateAccessToken(t.Context(), pair.AccessToken, "access-key")
		require.Error(t, err, "old access survived password change")
	}

	{
		_, err := tokens.Rotate(t.Context(), repo.user, pair.RefreshToken, repo)
		require.ErrorIs(t, err, ErrRefreshTokenInvalid, "old refresh survived password change: %v", err)
	}
}

func TestRefreshTokenRotationIsSingleUseAndReplayRevokesSuccessor(t *testing.T) {
	t.Parallel()

	tokens, reader := newSecurityTokens()
	repo := &securityAccountRepo{user: &domain.User{ID: 7, Username: testTester, Password: "hash", Status: enum.UserStatusActive}}

	original, err := tokens.IssueForUser(t.Context(), repo.user, repo)
	require.NoError(t, err)

	service := &LoginService{tokens: tokens, users: repo, logger: zap.NewNop(), cfgProvider: reader}

	next, err := service.RefreshToken(t.Context(), original.RefreshToken)
	require.NoError(t, err)

	require.NotEqual(t, next.RefreshToken, original.RefreshToken, "rotation reused JWT")
	require.NotEqual(t, next.AccessToken, original.AccessToken, "rotation reused JWT")

	{
		_, err := service.RefreshToken(t.Context(), original.RefreshToken)
		require.Error(t, err, "replayed token accepted")
	}

	{
		_, err := service.RefreshToken(t.Context(), next.RefreshToken)
		require.Error(t, err, "successor survived detected replay")
	}
}

func TestConcurrentRefreshHasOneWinner(t *testing.T) {
	t.Parallel()

	tokens, _ := newSecurityTokens()
	repo := &securityAccountRepo{user: &domain.User{ID: 7, Password: "hash"}}

	pair, err := tokens.IssueForUser(t.Context(), repo.user, repo)
	require.NoError(t, err)

	user := *repo.user

	var wg sync.WaitGroup

	outcomes := make(chan error, 2)

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := tokens.Rotate(context.Background(), &user, pair.RefreshToken, repo)
			outcomes <- err
		}()
	}

	wg.Wait()
	close(outcomes)

	successes := 0

	for err := range outcomes {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, ErrRefreshTokenReused, err)
		}
	}

	require.Equal(t, 1, successes, "successful refreshes = %d", successes)
}

func TestBanBlocksRefreshIncludingTemporaryBan(t *testing.T) {
	t.Parallel()

	for _, state := range []enum.UserStatus{enum.UserStatusBanned, enum.UserStatusPermanentlyBanned} {
		tokens, _ := newSecurityTokens()
		repo := &securityAccountRepo{user: &domain.User{ID: 7, Password: "hash", Status: state}}

		pair, err := tokens.IssueForUser(t.Context(), repo.user, repo)
		require.NoError(t, err)

		service := &LoginService{tokens: tokens, users: repo, logger: zap.NewNop()}
		{
			_, err := service.RefreshToken(t.Context(), pair.RefreshToken)
			require.ErrorIs(t, err, domain.ErrAccountBanned, "ban did not reject refresh: %v", err)
		}
	}
}

func TestOAuthCanAssociateExplicitlyVerifiedEmail(t *testing.T) {
	t.Parallel()

	repo := &securityAccountRepo{user: &domain.User{ID: 9, Email: "owner@example.com"}}
	service := &OAuthLoginService{users: repo}

	user, err := service.findOrCreateUser(t.Context(), domain.OAuthProfile{RegisterSource: OAuthProviderGitHub, OpenID: "owner", Email: "owner@example.com", EmailVerified: true})
	require.NoError(t, err, "verified association user=%v err=%v", user, err)
	require.NotNil(t, user, "verified association user=%v err=%v", user, err)
	require.EqualValues(t, 9, user.ID, "verified association user=%v err=%v", user, err)
}

func TestUnverifiedOAuthEmailUsesProviderFallback(t *testing.T) {
	t.Parallel()

	profile := domain.OAuthProfile{RegisterSource: OAuthProviderGitHub, OpenID: "attacker", Email: testVictimExampleCom}
	{
		got := profile.ResolvedEmail()
		require.NotEqual(t, testVictimExampleCom, got, "unverified email stored as owned")
	}
}
