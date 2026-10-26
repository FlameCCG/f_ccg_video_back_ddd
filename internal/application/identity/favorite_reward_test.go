package identity_test

import (
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/identity"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestFavoriteRewardAndClawbackAreLifetimeIdempotent(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "rewards.db")), &gorm.Config{TranslateError: true})
	require.NoError(t, err)

	conn, _ := db.DB()

	t.Cleanup(func() { _ = conn.Close() })

	{
		err = db.AutoMigrate(&po.User{}, &po.UserCoinRecord{})
		require.NoError(t, err)
	}

	{
		err = db.Create(&po.User{Model: po.Model{ID: 1}, Username: "author", Email: "author@example.com", Password: "hash"}).Error
		require.NoError(t, err)
	}

	s := app.NewCoinService(gormx.NewTxManager(db), infra.NewUserRepository(db, nil), infra.NewCoinRecordRepository(db))

	key := "favorite:2:10"

	for range 20 {
		{
			_, _, err = s.ApplyChange(t.Context(), 1, .1, "favorite", &key)
			require.NoError(t, err)
		}

		{
			err = s.RevokeFavoriteReward(t.Context(), 1, 2, 10)
			require.NoError(t, err)
		}
	}

	var user po.User
	db.First(&user, 1)

	require.InDelta(t, 0, user.CoinCount, 1e-9, "balance=%v", user.CoinCount)

	var n int64
	db.Model(&po.UserCoinRecord{}).Count(&n)

	require.EqualValues(t, 2, n, "ledger records=%d", n)

	{
		err = s.RevokeFavoriteReward(t.Context(), 1, 3, 10)
		require.NoError(t, err)
	}

	db.First(&user, 1)

	require.InDelta(t, 0, user.CoinCount, 1e-9, "unearned reward revoked")
	// Spent rewards can be clawed back without preventing future earnings or
	// making a duplicate lifetime reward fail as an insufficient-balance error.
	{
		err := db.Model(&po.User{}).Where("id=1").UpdateColumn("coin_count", -.3).Error
		require.NoError(t, err)
	}

	{
		_, applied, err := s.ApplyChange(t.Context(), 1, .1, "favorite", &key)
		require.NoError(t, err, "duplicate on negative balance: applied=%v err=%v", applied, err)
		require.False(t, applied, "duplicate on negative balance: applied=%v err=%v", applied, err)
	}

	{
		_, applied, err := s.ApplyChange(t.Context(), 1, .1, "new earning", nil)
		require.NoError(t, err, "debt recovery: applied=%v err=%v", applied, err)
		require.True(t, applied, "debt recovery: applied=%v err=%v", applied, err)
	}

	{
		err := db.First(&user, 1).Error
		require.NoError(t, err, "debt balance=%v err=%v", user.CoinCount, err)
		require.InDelta(t, -.2, user.CoinCount, 1e-9, "debt balance=%v err=%v", user.CoinCount, err)
	}
}
