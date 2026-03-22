package identity_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	identity "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

func TestCredentialActionsProtectRelatedState(t *testing.T) {
	u := identity.RestoreUser(identity.UserSnapshot{ID: 1, Password: "initial", RefreshTokenHash: "refresh", MustChangePassword: true, Email: "old@example.com", CurrentRoleID: 2, CoinCount: 3.4})
	before := u.Snapshot()
	for _, tc := range []struct {
		change identity.PasswordChange
		want   error
	}{
		{identity.PasswordChange{PasswordHash: "new"}, identity.ErrOldPasswordRequired},
		{identity.PasswordChange{OldPassword: "wrong", PasswordHash: "new"}, identity.ErrOldPasswordIncorrect},
		{identity.PasswordChange{OldPassword: "initial", OldPasswordMatches: true, NewPasswordMatches: true, PasswordHash: "new"}, identity.ErrInitialPasswordUnchanged},
	} {
		if err := u.ChangePassword(tc.change); !errors.Is(err, tc.want) || !reflect.DeepEqual(before, u.Snapshot()) {
			t.Fatalf("rejected change: err=%v user=%+v", err, u.Snapshot())
		}
	}
	if err := u.ChangePassword(identity.PasswordChange{OldPassword: "initial", OldPasswordMatches: true, PasswordHash: "new"}); err != nil {
		t.Fatal(err)
	}
	if u.Password() != "new" || u.RefreshTokenHash() != "" || u.MustChangePassword() || u.CoinCount() != 3.4 || u.Email() != before.Email {
		t.Fatalf("credential action changed unrelated state or left stale credentials: %+v", u.Snapshot())
	}
	if _, err := u.BindEmail("occupied@example.com", 1); !errors.Is(err, identity.ErrEmailOccupied) || u.Email() != before.Email {
		t.Fatal("occupied email changed state", err)
	}
	if already, err := u.BindEmail(before.Email, 1); err != nil || !already {
		t.Fatal("same email must remain idempotent", err)
	}
	if already, err := u.BindEmail("new@example.com", 0); err != nil || already || u.Email() != "new@example.com" {
		t.Fatal("email binding failed", err)
	}
	if err := u.SwitchRole(3, false); !errors.Is(err, identity.ErrRoleNotOwned) || u.CurrentRoleID() != 2 {
		t.Fatal("unowned role changed state", err)
	}
	if err := u.SwitchRole(3, true); err != nil || u.CurrentRoleID() != 3 {
		t.Fatal("owned role not selected", err)
	}

	oauth := identity.RestoreUser(identity.UserSnapshot{ID: 2, RefreshTokenHash: "refresh", MustChangePassword: true})
	if err := oauth.ChangePassword(identity.PasswordChange{PasswordHash: "first"}); err != nil || oauth.Password() != "first" || oauth.RefreshTokenHash() != "" || oauth.MustChangePassword() {
		t.Fatal("OAuth first password changed", err)
	}
}

func TestBanActionsPreserveHistoryAndReleaseAtomically(t *testing.T) {
	now := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	u := identity.RestoreUser(identity.UserSnapshot{ID: 1, Status: enum.UserStatusActive})
	before := u.Snapshot()
	if _, err := u.ApplyBan(identity.BanAction{UserID: 1, Status: enum.UserStatusBanned}, now); !errors.Is(err, identity.ErrBanDaysRequired) || !reflect.DeepEqual(before, u.Snapshot()) {
		t.Fatal("invalid ban changed state", err)
	}
	if record, err := identity.NewBanRecord(identity.BanAction{UserID: 1, Status: enum.UserStatusBanned}, now); record != nil || !errors.Is(err, identity.ErrBanDaysRequired) {
		t.Fatal("constructor bypassed ban validation", record, err)
	}
	ban, err := u.ApplyBan(identity.BanAction{UserID: 1, Status: enum.UserStatusBanned, Days: 1, Reason: " reason "}, now)
	if err != nil || !u.IsBanned() || ban.Reason() != "reason" || !ban.EndAt().Equal(now.AddDate(0, 0, 1)) {
		t.Fatal("ban facts not applied together", err)
	}
	if err := u.EnsureCanPublish(); !errors.Is(err, identity.ErrAccountBanned) {
		t.Fatal(err)
	}
	if u.ReleaseExpiredBan(ban, now) || ban.IsUnbanned() || !u.IsBanned() {
		t.Fatal("future ban must not be released")
	}
	if !u.ReleaseExpiredBan(ban, *ban.EndAt()) || !ban.IsUnbanned() || u.IsBanned() || ban.Status() != enum.UserStatusBanned || ban.Reason() != "reason" {
		t.Fatal("release lost history or did not update both states")
	}
	if u.ReleaseExpiredBan(ban, now.AddDate(0, 0, 2)) {
		t.Fatal("release must be idempotent")
	}
	if err := u.EnsureCanPublish(); err != nil {
		t.Fatal(err)
	}
	permanent, err := u.ApplyBan(identity.BanAction{UserID: 1, Status: enum.UserStatusPermanentlyBanned}, now)
	if err != nil || u.ReleaseExpiredBan(permanent, now.AddDate(1, 0, 0)) {
		t.Fatal("permanent ban cannot expire", err)
	}
}
