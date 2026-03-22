package identity

import (
	"context"
	"errors"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/identity"
	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/pkg/encrypt"
)

type accountRoleFacts struct {
	app.UserRoleQuery
	owned               bool
	transactionObserved bool
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
	service := app.NewAccountService(gormx.NewTxManager(db), repo, roles)
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
	if err := db.Exec(`UPDATE users SET refresh_token_hash = 'session', must_change_password = 1 WHERE id = 1`).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.ResetPassword(t.Context(), app.ResetPasswordInput{Email: "new@example.com", NewPassword: "replacement"}); err != nil {
		t.Fatal(err)
	}
	user, err = repo.FindByID(t.Context(), 1)
	if err != nil || !encrypt.ValidatePasswordHash("replacement", user.Password()) || user.RefreshTokenHash() != "" || user.MustChangePassword() {
		t.Fatal("reset did not clear all credential state", err)
	}
	if user.CoinCount() != 4.5 || user.CurrentRoleID() != 3 || user.Email() != "new@example.com" || user.Username() != "joined-user" || user.Avatar() != "avatar.png" {
		t.Fatal("field save overwrote unrelated state", user.Snapshot())
	}
}

func TestAccountCredentialSaveRollsBackAndPreservesFailureContract(t *testing.T) {
	db := openUserProfileTestDB(t)
	repo := NewUserRepository(db, nil)
	if err := db.Exec(`UPDATE users SET refresh_token_hash = 'session', must_change_password = 1 WHERE id = 1`).Error; err != nil {
		t.Fatal(err)
	}
	service := app.NewAccountService(abortAccountTransaction{gormx.NewTxManager(db)}, repo, nil)
	if err := service.ResetPassword(t.Context(), app.ResetPasswordInput{Email: "joined-user@example.com", NewPassword: "replacement"}); !errors.Is(err, app.ErrResetPasswordFailed) {
		t.Fatal("transaction failure changed public error", err)
	}
	user, err := repo.FindByID(t.Context(), 1)
	if err != nil || user.Password() != "password-hash" || user.RefreshTokenHash() != "session" || !user.MustChangePassword() {
		t.Fatal("failed transaction leaked credential changes", err)
	}
}
