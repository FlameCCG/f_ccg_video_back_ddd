package identity_test

import (
	"context"
	"path/filepath"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/config"
	domain "f_ccg_video_back_ddd/internal/domain/identity"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

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
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	if err := db.AutoMigrate(&po.User{}, &po.UserExpRecord{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	user := po.User{
		Username: "exp_user",
		Email:    "exp@example.com",
		Password: "x",
		Exp:      0,
		Level:    1,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}

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
	svc, db, userID := newExpFixture(t)
	ctx := context.Background()

	// 投币 +45、登录 +10，让真实奖励入口累计到 55。
	if err := svc.CoinExpForVideo(ctx, userID, 1, 3); err != nil {
		t.Fatalf("seed coin exp: %v", err)
	}
	if err := svc.LoginExp(ctx, userID); err != nil {
		t.Fatalf("seed login exp: %v", err)
	}

	// 点赞本应 +10，但日上限只剩 5
	if err := svc.LikeExp(ctx, userID); err != nil {
		t.Fatalf("LikeExp: %v", err)
	}

	var user po.User
	if err := db.First(&user, userID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}

	if user.Exp != domain.DailyExpCap {
		t.Fatalf("exp after like clamp = %d, want %d", user.Exp, domain.DailyExpCap)
	}

	// 已达上限：登录/观看/再点赞均不应再涨
	if err := svc.LoginExp(ctx, userID); err != nil {
		t.Fatalf("LoginExp: %v", err)
	}

	if err := svc.WatchExp(ctx, userID); err != nil {
		t.Fatalf("WatchExp: %v", err)
	}

	if err := svc.LikeExp(ctx, userID); err != nil {
		t.Fatalf("LikeExp at cap: %v", err)
	}

	if err := svc.CoinExpForVideo(ctx, userID, 2, 2); err != nil {
		t.Fatalf("CoinExpForVideo: %v", err)
	}

	if err := db.First(&user, userID).Error; err != nil {
		t.Fatalf("reload user after cap: %v", err)
	}

	if user.Exp != domain.DailyExpCap {
		t.Fatalf("exp after more ops = %d, want stay at %d", user.Exp, domain.DailyExpCap)
	}

	var gained int64
	if err := db.Model(&po.UserExpRecord{}).
		Select("COALESCE(SUM(delta), 0)").
		Where("user_id = ? AND delta > 0", userID).
		Scan(&gained).Error; err != nil {
		t.Fatalf("sum records: %v", err)
	}

	if gained != domain.DailyExpCap {
		t.Fatalf("sum of positive records = %d, want %d", gained, domain.DailyExpCap)
	}
}

func TestLikeExpDailyIdempotent(t *testing.T) {
	svc, db, userID := newExpFixture(t)
	ctx := context.Background()

	// 循环点赞：同一自然日仅首次加经验
	for i := range 5 {
		if err := svc.LikeExp(ctx, userID); err != nil {
			t.Fatalf("LikeExp #%d: %v", i, err)
		}
	}

	var user po.User
	if err := db.First(&user, userID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}

	if user.Exp != domain.ExpPerLike {
		t.Fatalf("exp after loop = %d, want %d (daily idempotent)", user.Exp, domain.ExpPerLike)
	}

	var recCount int64
	if err := db.Model(&po.UserExpRecord{}).
		Where("user_id = ? AND reason LIKE ?", userID, "点赞%").
		Count(&recCount).Error; err != nil {
		t.Fatalf("count records: %v", err)
	}

	if recCount != 1 {
		t.Fatalf("like exp records = %d, want 1", recCount)
	}
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
		if got := domain.CalcLevel(exp); got != want {
			t.Fatalf("CalcLevel(%d) = %d, want %d", exp, got, want)
		}
	}
}

// TestClampDailyExpGain 每日经验上限裁剪规则。
func TestClampDailyExpGain(t *testing.T) {
	t.Parallel()

	if applied, allowed := domain.ClampDailyExpGain(-5, 60); !allowed || applied != -5 {
		t.Fatalf("negative delta should pass through, got (%d, %v)", applied, allowed)
	}

	if applied, allowed := domain.ClampDailyExpGain(10, 55); !allowed || applied != 5 {
		t.Fatalf("expected clamp to 5, got (%d, %v)", applied, allowed)
	}

	if _, allowed := domain.ClampDailyExpGain(10, domain.DailyExpCap); allowed {
		t.Fatal("expected not allowed when daily cap reached")
	}

	if applied, allowed := domain.ClampDailyExpGain(10, 0); !allowed || applied != 10 {
		t.Fatalf("expected full delta, got (%d, %v)", applied, allowed)
	}
}
