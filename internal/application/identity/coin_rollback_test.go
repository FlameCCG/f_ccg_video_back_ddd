package identity_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/domain/identity/entity"
	"f_ccg_video_back_ddd/internal/domain/identity/repository"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type failingCoinSave struct {
	repository.CoinAccountRepository
	failure error
}

func (r failingCoinSave) SaveBalance(ctx context.Context, a *entity.CoinAccount) error {
	if err := r.CoinAccountRepository.SaveBalance(ctx, a); err != nil {
		return err
	}
	return r.failure
}

func TestCoinAccountBalanceAndLedgerRollbackTogether(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "coins.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.AutoMigrate(&po.User{}, &po.UserCoinRecord{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(&po.User{Model: po.Model{ID: 1}, Username: "coin", Email: "coin@example.com", CoinCount: 1}).Error; err != nil {
		t.Fatal(err)
	}
	txm := gormx.NewTxManager(db)
	accounts := infra.NewCoinAccountRepository(db)
	records := infra.NewCoinRecordRepository(db)
	failure := errors.New("save failed")
	service := app.NewCoinService(txm, failingCoinSave{accounts, failure}, records)
	result, err := service.ApplyChange(t.Context(), 1, -.24, "spend", nil)
	if !errors.Is(err, failure) || result.Applied {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	service = app.NewCoinService(txm, accounts, records)
	err = txm.Do(t.Context(), func(ctx context.Context) error {
		result, err := service.ApplyChange(ctx, 1, -.24, "spend", nil)
		if err != nil || !result.Applied || result.UserID != 1 || result.Balance != .8 {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	var user po.User
	var count int64
	if err := db.First(&user, 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&po.UserCoinRecord{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if user.CoinCount != 1 || count != 0 {
		t.Fatalf("balance=%v records=%d", user.CoinCount, count)
	}
}
