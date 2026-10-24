package identity_test

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func TestFavoriteRewardAndClawbackAreLifetimeIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "rewards.db")), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	t.Cleanup(func() { _ = conn.Close() })
	if err = db.AutoMigrate(&po.User{}, &po.UserCoinRecord{}); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&po.User{Model: po.Model{ID: 1}, Username: "author", Email: "author@example.com", Password: "hash"}).Error; err != nil {
		t.Fatal(err)
	}
	s := app.NewCoinService(gormx.NewTxManager(db), infra.NewUserRepository(db, nil), infra.NewCoinRecordRepository(db))
	key := "favorite:2:10"
	for i := 0; i < 20; i++ {
		if _, _, err = s.ApplyChange(t.Context(), 1, .1, "favorite", &key); err != nil {
			t.Fatal(err)
		}
		if err = s.RevokeFavoriteReward(t.Context(), 1, 2, 10); err != nil {
			t.Fatal(err)
		}
	}
	var user po.User
	db.First(&user, 1)
	if user.CoinCount != 0 {
		t.Fatalf("balance=%v", user.CoinCount)
	}
	var n int64
	db.Model(&po.UserCoinRecord{}).Count(&n)
	if n != 2 {
		t.Fatalf("ledger records=%d", n)
	}
	if err = s.RevokeFavoriteReward(t.Context(), 1, 3, 10); err != nil {
		t.Fatal(err)
	}
	db.First(&user, 1)
	if user.CoinCount != 0 {
		t.Fatal("unearned reward revoked")
	}
	// Spent rewards can be clawed back without preventing future earnings or
	// making a duplicate lifetime reward fail as an insufficient-balance error.
	if err := db.Model(&po.User{}).Where("id=1").UpdateColumn("coin_count", -.3).Error; err != nil {
		t.Fatal(err)
	}
	if _, applied, err := s.ApplyChange(t.Context(), 1, .1, "favorite", &key); err != nil || applied {
		t.Fatalf("duplicate on negative balance: applied=%v err=%v", applied, err)
	}
	if _, applied, err := s.ApplyChange(t.Context(), 1, .1, "new earning", nil); err != nil || !applied {
		t.Fatalf("debt recovery: applied=%v err=%v", applied, err)
	}
	if err := db.First(&user, 1).Error; err != nil || user.CoinCount != -.2 {
		t.Fatalf("debt balance=%v err=%v", user.CoinCount, err)
	}
}
