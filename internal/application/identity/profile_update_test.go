package identity

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
)

type profileWriteTxKey struct{}

// This store models a row lock and rollback, including the author projection.
type profileWriteStore struct {
	domain.UserAccountRepository
	mu, txMu                                         sync.Mutex
	user                                             domain.UserSnapshot
	author                                           string
	readErr, saveErr, renameErr, beginErr, commitErr error
	occupied                                         int64
	ready                                            chan<- struct{}
	release                                          <-chan struct{}
}

func (s *profileWriteStore) FindByID(context.Context, uint) (*domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.user.ID == 0 {
		return nil, s.readErr
	}
	return domain.RestoreUser(s.user), s.readErr
}

func (s *profileWriteStore) FindByIDForUpdate(ctx context.Context, id uint) (*domain.User, error) {
	if ctx.Value(profileWriteTxKey{}) != s {
		return nil, errors.New("user lock lost transaction context")
	}
	return s.FindByID(ctx, id)
}

func (s *profileWriteStore) CountByUsernameExcluding(ctx context.Context, _ string, _ uint) (int64, error) {
	if ctx.Value(profileWriteTxKey{}) != s {
		return 0, errors.New("username facts lost transaction context")
	}
	return s.occupied, nil
}

func (s *profileWriteStore) UpdateProfile(ctx context.Context, _ uint, changes domain.ProfileChanges) error {
	if ctx.Value(profileWriteTxKey{}) != s {
		return errors.New("profile save lost transaction context")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if changes.Username != nil {
		s.user.Username = *changes.Username
	}
	if changes.Avatar != nil {
		s.user.Avatar = *changes.Avatar
	}
	return s.saveErr
}

func (s *profileWriteStore) RenameAuthor(ctx context.Context, oldName, newName string) error {
	if ctx.Value(profileWriteTxKey{}) != s {
		return errors.New("author rename lost transaction context")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.author == oldName {
		s.author = newName
	}
	return s.renameErr
}

func (s *profileWriteStore) Do(ctx context.Context, fn func(context.Context) error) error {
	if s.beginErr != nil {
		return s.beginErr
	}
	if s.ready != nil {
		s.ready <- struct{}{}
		select {
		case <-s.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.txMu.Lock()
	defer s.txMu.Unlock()
	s.mu.Lock()
	user, author := s.user, s.author
	s.mu.Unlock()
	err := fn(context.WithValue(ctx, profileWriteTxKey{}, s))
	if err == nil {
		err = s.commitErr
	}
	if err != nil {
		s.mu.Lock()
		s.user, s.author = user, author
		s.mu.Unlock()
	}
	return err
}

type profileWriteCache struct{ invalidations atomic.Int32 }

func (c *profileWriteCache) InvalidateUsername(uint) { c.invalidations.Add(1) }
func (c *profileWriteCache) InvalidateAuthorVideos(context.Context, string, bool) {
	c.invalidations.Add(1)
}

func TestProfileUpdateConcurrentRenamesKeepAuthorConsistent(t *testing.T) {
	ready, release := make(chan struct{}, 2), make(chan struct{})
	store := &profileWriteStore{
		user:   domain.UserSnapshot{ID: 1, Username: "A", CoinCount: 4.5, Avatar: "avatar.png"},
		author: "A", ready: ready, release: release,
	}
	service := NewProfileService(nil, store, store, nil, nil, nil, nil, nil, store, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	errs := make(chan error, 2)
	for _, name := range []string{"B", "C"} {
		go func() {
			errs <- service.UpdateProfile(ctx, UpdateProfileInput{UserID: 1, Username: name})
		}()
	}
	// Both requests reach transaction entry before either can modify the user.
	for range 2 {
		select {
		case <-ready:
		case <-ctx.Done():
			t.Fatal("concurrent updates did not reach transaction entry", ctx.Err())
		}
	}
	close(release)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if store.user.Username != store.author || store.user.Username == "A" {
		t.Fatalf("user=%q author=%q", store.user.Username, store.author)
	}
	if store.user.CoinCount != 4.5 || store.user.Avatar != "avatar.png" {
		t.Fatal("rename overwrote unrelated profile state", store.user)
	}
}

func TestProfileUpdateFailurePreservesStateAndErrorContract(t *testing.T) {
	failure := errors.New("storage failure")
	for _, tc := range []struct {
		name      string
		configure func(*profileWriteStore, *UpdateProfileInput)
		want      error
	}{
		{"read", func(s *profileWriteStore, _ *UpdateProfileInput) { s.readErr = failure }, domain.ErrUserNotFound},
		{"missing", func(s *profileWriteStore, _ *UpdateProfileInput) { s.user.ID = 0 }, domain.ErrUserNotFound},
		{"occupied", func(s *profileWriteStore, _ *UpdateProfileInput) { s.occupied = 1 }, domain.ErrUsernameExists},
		{"empty", func(_ *profileWriteStore, in *UpdateProfileInput) { in.Username = "" }, domain.ErrNoFieldsToUpdate},
		{"save", func(s *profileWriteStore, _ *UpdateProfileInput) { s.saveErr = failure }, ErrUpdateUserFailed},
		{"author", func(s *profileWriteStore, _ *UpdateProfileInput) { s.renameErr = failure }, ErrUpdateUserFailed},
		{"begin", func(s *profileWriteStore, _ *UpdateProfileInput) { s.beginErr = failure }, ErrUpdateUserFailed},
		{"commit", func(s *profileWriteStore, _ *UpdateProfileInput) { s.commitErr = failure }, ErrUpdateUserFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &profileWriteStore{user: domain.UserSnapshot{ID: 1, Username: "A"}, author: "A"}
			input := UpdateProfileInput{UserID: 1, Username: "B"}
			tc.configure(store, &input)
			cache := &profileWriteCache{}
			service := NewProfileService(nil, store, store, nil, nil, nil, nil, nil, store, cache)
			if err := service.UpdateProfile(t.Context(), input); !errors.Is(err, tc.want) {
				t.Fatalf("error=%v, want %v", err, tc.want)
			}
			if store.user.Username != "A" || store.author != "A" || cache.invalidations.Load() != 0 {
				t.Fatalf("failed update changed state: user=%q author=%q invalidations=%d", store.user.Username, store.author, cache.invalidations.Load())
			}
		})
	}
}
