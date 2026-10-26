package identity_test

import (
	"context"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/identity"

	domain "f_ccg_video_back_ddd/internal/domain/identity"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newExpFixture 用 sqlite 临时库装配一套真实仓储 + 应用服务（测试不走 wire）。
func newExpFixture(t *testing.T) (*app.ExpService, *gorm.DB, uint) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "exp-test.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "open sqlite failed: %v", err)

	{
		err := db.AutoMigrate(&po.User{}, &po.UserExpRecord{})
		require.NoError(t, err, "migrate failed: %v", err)
	}

	user := po.User{
		Username: "exp_user",
		Email:    "exp@example.com",
		Password: "x",
		Exp:      0,
		Level:    1,
	}
	// 跳过 AfterCreate（会写 user_confs / 默认收藏夹等），本测只需 exp 表。
	{
		err := db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error
		require.NoError(t, err, "create user failed: %v", err)
	}

	sqlDB, err := db.DB()
	require.NoError(t, err, "db.DB: %v", err)

	t.Cleanup(func() { _ = sqlDB.Close() })

	cfgProvider := infraconfig.NewProvider(&config.Config{})
	svc := app.NewExpService(
		gormx.NewTxManager(db),
		infra.NewUserRepository(db, cfgProvider),
		infra.NewExpRecordRepository(db),
	)

	return svc, db, user.ID
}

func TestDailyExpCapAppliesToAllGains(t *testing.T) {
	t.Parallel()

	svc, db, userID := newExpFixture(t)
	ctx := context.Background()

	// 先加到接近上限：55
	{
		_, err := svc.AddExp(ctx, userID, 55, "seed")
		require.NoError(t, err, "seed exp: %v", err)
	}

	// 点赞本应 +10，但日上限只剩 5
	{
		err := svc.LikeExp(ctx, userID)
		require.NoError(t, err, "LikeExp: %v", err)
	}

	var user po.User
	{
		err := db.First(&user, userID).Error
		require.NoError(t, err, "reload user: %v", err)
	}

	require.EqualValues(t, user.Exp, domain.DailyExpCap, "exp after like clamp = %d, want %d", user.Exp, domain.DailyExpCap)

	// 已达上限：登录/观看/再点赞均不应再涨
	{
		err := svc.LoginExp(ctx, userID)
		require.NoError(t, err, "LoginExp: %v", err)
	}

	{
		err := svc.WatchExp(ctx, userID)
		require.NoError(t, err, "WatchExp: %v", err)
	}

	{
		err := svc.LikeExp(ctx, userID)
		require.NoError(t, err, "LikeExp at cap: %v", err)
	}

	{
		err := svc.CoinExp(ctx, userID, 2)
		require.NoError(t, err, "CoinExp: %v", err)
	}

	{
		err := db.First(&user, userID).Error
		require.NoError(t, err, "reload user after cap: %v", err)
	}

	require.EqualValues(t, user.Exp, domain.DailyExpCap, "exp after more ops = %d, want stay at %d", user.Exp, domain.DailyExpCap)

	var gained int64
	{
		err := db.Model(&po.UserExpRecord{}).
			Select("COALESCE(SUM(delta), 0)").
			Where("user_id = ? AND delta > 0", userID).
			Scan(&gained).Error
		require.NoError(t, err, "sum records: %v", err)
	}

	require.EqualValues(t, domain.DailyExpCap, gained, "sum of positive records = %d, want %d", gained, domain.DailyExpCap)
}

func TestLikeExpDailyIdempotent(t *testing.T) {
	t.Parallel()

	svc, db, userID := newExpFixture(t)
	ctx := context.Background()

	// 循环点赞：同一自然日仅首次加经验
	for i := range 5 {
		{
			err := svc.LikeExp(ctx, userID)
			require.NoError(t, err, "LikeExp #%d: %v", i, err)
		}
	}

	var user po.User
	{
		err := db.First(&user, userID).Error
		require.NoError(t, err, "reload user: %v", err)
	}

	require.EqualValues(t, user.Exp, domain.ExpPerLike, "exp after loop = %d, want %d (daily idempotent)", user.Exp, domain.ExpPerLike)

	var recCount int64
	{
		err := db.Model(&po.UserExpRecord{}).
			Where("user_id = ? AND reason LIKE ?", userID, "点赞%").
			Count(&recCount).Error
		require.NoError(t, err, "count records: %v", err)
	}

	require.EqualValues(t, 1, recCount, "like exp records = %d, want 1", recCount)
}

// TestCalcLevel 领域等级换算（原 calcLevel 私有函数的行为，改为领域层可测公开函数）。
func TestCalcLevel(t *testing.T) {
	t.Parallel()

	cases := map[int64]int{
		0:     1,
		999:   1,
		1000:  2,
		2999:  2,
		3000:  3,
		5999:  3,
		6000:  4,
		9999:  4,
		10000: 5,
		19999: 5,
		20000: 6,
		99999: 6,
	}

	for exp, want := range cases {
		{
			got := domain.CalcLevel(exp)
			require.Equal(t, want, got, "CalcLevel(%d) = %d, want %d", exp, got, want)
		}
	}
}

// TestClampDailyExpGain 每日经验上限裁剪规则。
func TestClampDailyExpGain(t *testing.T) {
	t.Parallel()

	{
		applied, allowed := domain.ClampDailyExpGain(-5, 60)
		require.True(t, allowed, "negative delta should pass through, got (%d, %v)", applied, allowed)
		require.EqualValues(t, -5, applied, "negative delta should pass through, got (%d, %v)", applied, allowed)
	}

	{
		applied, allowed := domain.ClampDailyExpGain(10, 55)
		require.True(t, allowed, "expected clamp to 5, got (%d, %v)", applied, allowed)
		require.EqualValues(t, 5, applied, "expected clamp to 5, got (%d, %v)", applied, allowed)
	}

	{
		_, allowed := domain.ClampDailyExpGain(10, domain.DailyExpCap)
		require.False(t, allowed, "expected not allowed when daily cap reached")
	}

	{
		applied, allowed := domain.ClampDailyExpGain(10, 0)
		require.True(t, allowed, "expected full delta, got (%d, %v)", applied, allowed)
		require.EqualValues(t, 10, applied, "expected full delta, got (%d, %v)", applied, allowed)
	}
}
