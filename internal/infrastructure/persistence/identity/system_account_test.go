package identity

import (
	"context"
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"gorm.io/gorm"
	"testing"
)

func TestSystemAccountUpdateDoesNotOverwriteIndependentFields(t *testing.T) {
	db := openUserProfileTestDB(t)
	repo := NewUserRepository(db, nil)
	service := app.NewSystemAccountService(repo, nil)
	changed := false
	if err := db.Callback().Query().After("gorm:query").Register("test:independent_account_write", func(q *gorm.DB) {
		if changed || q.Statement.Table != "users" {
			return
		}
		changed = true
		q.AddError(q.Session(&gorm.Session{NewDB: true}).Exec("UPDATE users SET coin_count=25,status=2,avatar='new-avatar' WHERE id=1").Error)
	}); err != nil {
		t.Fatal(err)
	}
	err := gormx.NewTxManager(db).Do(t.Context(), func(ctx context.Context) error {
		id, created, err := service.UpsertSuperUser(ctx, "joined-user", "next-hash", 9)
		if err == nil && (id != 1 || created) {
			t.Fatalf("id=%d created=%v", id, created)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var saved po.User
	if err := db.First(&saved, 1).Error; err != nil {
		t.Fatal(err)
	}
	if !changed || saved.CoinCount != 25 || saved.Status != 2 || saved.Avatar != "new-avatar" {
		t.Fatalf("unrelated state overwritten: %+v", saved)
	}
	if saved.Password != "next-hash" || !saved.MustChangePassword || saved.RefreshTokenHash != "" || saved.Level != 6 || saved.Exp != 20000 || saved.CurrentRoleID != 9 {
		t.Fatalf("system account transition not saved: %+v", saved)
	}
}
