package identity

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/authsession"
	jwts "f_ccg_video_back_ddd/pkg/jwt"
)

type validationUsers struct {
	identitydomain.UserRepository
	user *identitydomain.User
	err  error
}

func (r validationUsers) FindByID(context.Context, uint) (*identitydomain.User, error) {
	return r.user, r.err
}

type validationSessions struct {
	SessionStore
	found bool
	err   error
}

func (s validationSessions) Exists(context.Context, uint, string) (bool, error) {
	return s.found, s.err
}

func TestAccessValidationDistinguishesInvalidSessionsFromLookupFailures(t *testing.T) {
	t.Parallel()
	lookupFailure := errors.New("database unavailable")
	sessionFailure := errors.New("Redis unavailable")
	for _, tc := range []struct {
		name         string
		invalidToken bool
		invalidUID   bool
		missingUser  bool
		password     string
		status       enum.UserStatus
		revoked      bool
		missingStore bool
		lookupErr    error
		sessionErr   error
		wantInvalid  bool
		wantCause    error
	}{
		{name: "valid"},
		{name: "malformed token", invalidToken: true, wantInvalid: true},
		{name: "invalid UID", invalidUID: true, wantInvalid: true},
		{name: "missing user", missingUser: true, wantInvalid: true, wantCause: identitydomain.ErrUserNotFound},
		{name: "changed password", password: "new-hash", wantInvalid: true},
		{name: "banned user", status: enum.UserStatusPermanentlyBanned, wantInvalid: true, wantCause: identitydomain.ErrAccountBanned},
		{name: "revoked session", revoked: true, wantInvalid: true, wantCause: identitydomain.ErrUserNotLogin},
		{name: "missing store", missingStore: true},
		{name: "database deadline", lookupErr: fmt.Errorf("query: %w", context.DeadlineExceeded), wantCause: context.DeadlineExceeded},
		{name: "session deadline", sessionErr: fmt.Errorf("session: %w", context.DeadlineExceeded), wantCause: context.DeadlineExceeded},
		{name: "database failure", lookupErr: lookupFailure, wantCause: lookupFailure},
		{name: "session failure", sessionErr: sessionFailure, wantCause: sessionFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uid := "7"
			if tc.invalidUID {
				uid = "invalid"
			}
			pair, err := jwts.CreateJwtTokenWithPassword(uid, "tester", "old-hash", "session", time.Hour, "access", time.Hour, "refresh")
			if err != nil {
				t.Fatal(err)
			}
			token := pair.AccessToken
			if tc.invalidToken {
				token = "invalid"
			}
			password := tc.password
			if password == "" {
				password = "old-hash"
			}
			status := tc.status
			if status == 0 {
				status = enum.UserStatusActive
			}
			user := identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 7, Username: "tester", Password: password, Status: status})
			if tc.missingUser {
				user = nil
			}
			var sessions SessionStore = validationSessions{found: !tc.revoked, err: tc.sessionErr}
			if tc.missingStore {
				sessions = nil
			}
			service := NewUserStatusService(nil, validationUsers{user: user, err: tc.lookupErr}, sessions)
			got, err := service.ValidateAccessToken(t.Context(), token, "access")
			if tc.name == "valid" {
				if err != nil || got == nil || got.ID() != 7 {
					t.Fatalf("valid session rejected: %v", err)
				}
				return
			}
			if err == nil || got != nil || errors.Is(err, authsession.ErrInvalidSession) != tc.wantInvalid {
				t.Fatalf("user=%v error=%v want invalid=%v", got, err, tc.wantInvalid)
			}
			if tc.wantCause != nil && !errors.Is(err, tc.wantCause) {
				t.Fatalf("error=%v lost cause=%v", err, tc.wantCause)
			}
		})
	}
}
