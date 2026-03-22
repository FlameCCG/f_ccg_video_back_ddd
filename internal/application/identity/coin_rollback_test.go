package identity_test

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/identity"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type failingCoinSave struct {
	identitydomain.CoinAccountRepository
	failure error
}

func TestRoundedCoinAmountsMatchLedgerAndDuplicateChangesStayUnapplied(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "rounded-coins.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.AutoMigrate(&po.User{}, &po.UserCoinRecord{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(&po.User{Model: po.Model{ID: 1}, Username: "rounding", Email: "rounding@example.com", CoinCount: 1}).Error; err != nil {
		t.Fatal(err)
	}
	records := infra.NewCoinRecordRepository(db)
	service := app.NewCoinService(gormx.NewTxManager(db), infra.NewCoinAccountRepository(db), records)
	key := "rounding"
	for _, step := range []struct {
		amount, balance float64
		applied         bool
		key             *string
	}{{.24, 1.2, true, &key}, {.05, 1.3, true, nil}, {-.15, 1.1, true, nil}, {-.04, 0, false, nil}, {.24, 1.1, false, &key}} {
		result, err := service.ApplyChange(t.Context(), 1, step.amount, "rounded", step.key)
		if err != nil || result.Balance != step.balance || result.Applied != step.applied {
			t.Fatalf("amount=%v result=%+v err=%v", step.amount, result, err)
		}
	}
	if _, err := service.ApplyChange(t.Context(), 1, -5, "insufficient", nil); !errors.Is(err, identitydomain.ErrCoinInsufficient) {
		t.Fatal(err)
	}
	var rows []po.UserCoinRecord
	if err := db.Order("id").Find(&rows).Error; err != nil || len(rows) != 3 {
		t.Fatalf("unexpected ledger: %+v err=%v", rows, err)
	}
	sum := 0.0
	for i, want := range []float64{.2, .1, -.2} {
		if rows[i].Delta != want {
			t.Fatalf("ledger row %d amount=%v want=%v", i, rows[i].Delta, want)
		}
		sum += rows[i].Delta
	}
	var user po.User
	if err := db.First(&user, 1).Error; err != nil || math.Abs(user.CoinCount-1-sum) > 1e-9 {
		t.Fatalf("balance and ledger diverged: balance=%v sum=%v err=%v", user.CoinCount, sum, err)
	}
	// 单独流水用例复用同一金额规则，不修改账户余额。
	if err := app.NewCoinRecordService(records, nil).Record(t.Context(), 1, .24, "record only"); err != nil {
		t.Fatal(err)
	}
	var last po.UserCoinRecord
	if err := db.Last(&last).Error; err != nil || last.Delta != .2 {
		t.Fatalf("standalone ledger precision changed: %+v %v", last, err)
	}
}

func (r failingCoinSave) SaveBalance(ctx context.Context, a *identitydomain.CoinAccount) error {
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
