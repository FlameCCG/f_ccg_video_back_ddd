package identity

import (
	"context"
	"errors"
	"sync"
	"testing"

	"f_ccg_video_back_ddd/internal/config"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"

	"go.uber.org/zap"
)

type securityAccountRepo struct {
	identitydomain.UserAccountRepository
	mu   sync.Mutex
	user *identitydomain.User
}

func (r *securityAccountRepo) FindByID(context.Context, uint) (*identitydomain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u := *r.user
	return &u, nil
}
func (r *securityAccountRepo) FindByOpenIDAndSource(context.Context, string, string) (*identitydomain.User, error) {
	return nil, nil
}
func (r *securityAccountRepo) FindByEmail(context.Context, string) (*identitydomain.User, error) {
	return r.user, nil
}
func (r *securityAccountRepo) SetRefreshToken(_ context.Context, id uint, pwd, digest string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.user.ID() != id || r.user.Password() != pwd {
		return false, nil
	}
	snap := r.user.Snapshot()
	snap.RefreshTokenHash = digest
	*r.user = *identitydomain.RestoreUser(snap)
	return true, nil
}
func (r *securityAccountRepo) RotateRefreshToken(_ context.Context, id uint, pwd, old, next string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.user.ID() != id || r.user.Password() != pwd || r.user.RefreshTokenHash() != old {
		return false, nil
	}
	snap := r.user.Snapshot()
	snap.RefreshTokenHash = next
	*r.user = *identitydomain.RestoreUser(snap)
	return true, nil
}

type securityStatusRepo struct {
	identitydomain.UserRepository
	account *securityAccountRepo
}

func (r securityStatusRepo) FindByID(ctx context.Context, id uint) (*identitydomain.User, error) {
	return r.account.FindByID(ctx, id)
}
func newSecurityTokens() (*TokenService, loginConfigReaderStub) {
	reader := loginConfigReaderStub{config: &config.Config{Jwt: &config.Jwt{AccessExp: 1, RefreshExp: 1, AccessSecret: "access-key", RefreshSecret: "refresh-key"}}}
	return NewTokenService(reader), reader
}
func TestOAuthDoesNotLinkPreexistingEmailAccount(t *testing.T) {
	repo := &securityAccountRepo{user: identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 9, Email: "victim@example.com"})}
	service := &OAuthLoginService{users: repo}
	user, err := service.findOrCreateUser(t.Context(), identitydomain.OAuthProfile{RegisterSource: "github", OpenID: "attacker", Email: "victim@example.com"})
	if !errors.Is(err, ErrOAuthAccountBindingRequired) || user != nil {
		t.Fatalf("account linked: user=%v error=%v", user, err)
	}
}
func TestPasswordChangeRevokesAccessAndRefreshAcrossServices(t *testing.T) {
	tokens, _ := newSecurityTokens()
	repo := &securityAccountRepo{user: identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 7, Username: "tester", Password: "salted-old-hash", Status: enum.UserStatusActive})}
	pair, err := tokens.IssueForUser(t.Context(), repo.user, repo)
	if err != nil {
		t.Fatal(err)
	}
	status := NewUserStatusService(nil, securityStatusRepo{account: repo})
	if _, err := status.ValidateAccessToken(t.Context(), pair.AccessToken, "access-key"); err != nil {
		t.Fatal(err)
	}
	snap := repo.user.Snapshot()
	snap.Password = "salted-new-hash"
	*repo.user = *identitydomain.RestoreUser(snap)
	if _, err := status.ValidateAccessToken(t.Context(), pair.AccessToken, "access-key"); err == nil {
		t.Fatal("old access survived password change")
	}
	if _, err := tokens.Rotate(t.Context(), repo.user, pair.RefreshToken, repo); !errors.Is(err, ErrRefreshTokenInvalid) {
		t.Fatalf("old refresh survived password change: %v", err)
	}
}
func TestRefreshTokenRotationIsSingleUseAndReplayRevokesSuccessor(t *testing.T) {
	tokens, reader := newSecurityTokens()
	repo := &securityAccountRepo{user: identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 7, Username: "tester", Password: "hash", Status: enum.UserStatusActive})}
	original, err := tokens.IssueForUser(t.Context(), repo.user, repo)
	if err != nil {
		t.Fatal(err)
	}
	service := &LoginService{tokens: tokens, users: repo, logger: zap.NewNop(), cfgProvider: reader}
	next, err := service.RefreshToken(t.Context(), original.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if next.RefreshToken == original.RefreshToken || next.AccessToken == original.AccessToken {
		t.Fatal("rotation reused JWT")
	}
	if _, err := service.RefreshToken(t.Context(), original.RefreshToken); err == nil {
		t.Fatal("replayed token accepted")
	}
	if _, err := service.RefreshToken(t.Context(), next.RefreshToken); err == nil {
		t.Fatal("successor survived detected replay")
	}
}
func TestConcurrentRefreshHasOneWinner(t *testing.T) {
	tokens, _ := newSecurityTokens()
	repo := &securityAccountRepo{user: identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 7, Password: "hash"})}
	pair, err := tokens.IssueForUser(t.Context(), repo.user, repo)
	if err != nil {
		t.Fatal(err)
	}
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
		} else if !errors.Is(err, ErrRefreshTokenReused) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful refreshes = %d", successes)
	}
}
func TestBanBlocksRefreshIncludingTemporaryBan(t *testing.T) {
	for _, state := range []enum.UserStatus{enum.UserStatusBanned, enum.UserStatusPermanentlyBanned} {
		tokens, _ := newSecurityTokens()
		repo := &securityAccountRepo{user: identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 7, Password: "hash", Status: state})}
		pair, err := tokens.IssueForUser(t.Context(), repo.user, repo)
		if err != nil {
			t.Fatal(err)
		}
		service := &LoginService{tokens: tokens, users: repo, logger: zap.NewNop()}
		if _, err := service.RefreshToken(t.Context(), pair.RefreshToken); !errors.Is(err, identitydomain.ErrAccountBanned) {
			t.Fatalf("ban did not reject refresh: %v", err)
		}
	}
}

func TestOAuthCanAssociateExplicitlyVerifiedEmail(t *testing.T) {
	repo := &securityAccountRepo{user: identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 9, Email: "owner@example.com"})}
	service := &OAuthLoginService{users: repo}
	user, err := service.findOrCreateUser(t.Context(), identitydomain.OAuthProfile{RegisterSource: "github", OpenID: "owner", Email: "owner@example.com", EmailVerified: true})
	if err != nil || user == nil || user.ID() != 9 {
		t.Fatalf("verified association user=%v err=%v", user, err)
	}
}
func TestUnverifiedOAuthEmailUsesProviderFallback(t *testing.T) {
	profile := identitydomain.OAuthProfile{RegisterSource: "github", OpenID: "attacker", Email: "victim@example.com"}
	if got := profile.ResolvedEmail(); got == "victim@example.com" {
		t.Fatal("unverified email stored as owned")
	}
}
