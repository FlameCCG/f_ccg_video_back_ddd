package scheduler

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	app "f_ccg_video_back_ddd/internal/application/platform"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// statusWriteRecorder 记录 write-through 的用户状态缓存写入，替代 identity 上下文的真实实现。
type statusWriteRecorder struct {
	writes map[uint]enum.UserStatus
}

func newStatusWriteRecorder() *statusWriteRecorder {
	return &statusWriteRecorder{writes: make(map[uint]enum.UserStatus)}
}

func (r *statusWriteRecorder) Write(userID uint, status enum.UserStatus) {
	r.writes[userID] = status
}

var _ app.UserStatusWriter = (*statusWriteRecorder)(nil)

func openAutoUnbanTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cron-unban-test.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	statements := []string{
		`CREATE TABLE users (
			id integer PRIMARY KEY AUTOINCREMENT,
			created_at datetime,
			updated_at datetime,
			status integer DEFAULT 1
		)`,
		`CREATE TABLE user_ban_records (
			id integer PRIMARY KEY AUTOINCREMENT,
			created_at datetime,
			updated_at datetime,
			user_id integer NOT NULL,
			status integer NOT NULL,
			reason text,
			days integer DEFAULT 0,
			start_at datetime,
			end_at datetime,
			operator_id integer DEFAULT 0,
			is_unbanned numeric NOT NULL DEFAULT 0
		)`,
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test tables failed: %v", err)
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db failed: %v", err)
	}

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}

func TestAutoUnbanReleasesOnlyExpiredBans(t *testing.T) {
	db := openAutoUnbanTestDB(t)
	now := time.Now()

	// 1：限时封禁已到期 → 应解封。
	// 2：限时封禁未到期 → 保持封禁。
	// 3：永久封禁（end_at 为 NULL）→ 保持封禁。
	// 4：最新记录已是解封（用户状态正常）→ 不重复处理。
	seedBanUser(t, db, 1, enum.UserStatusBanned, ptrTime(now.Add(-time.Hour)))
	seedBanUser(t, db, 2, enum.UserStatusBanned, ptrTime(now.Add(time.Hour)))
	seedBanUser(t, db, 3, enum.UserStatusPermanentlyBanned, nil)
	seedBanUser(t, db, 4, enum.UserStatusActive, nil)

	recorder := newStatusWriteRecorder()
	service := app.NewAutoUnbanService(
		gormx.NewTxManager(db),
		NewBanMaintenanceRepository(db),
		recorder,
		zap.NewNop(),
	)

	service.Run(context.Background())

	if got := userStatus(t, db, 1); got != enum.UserStatusActive {
		t.Fatalf("expected user 1 active, got %d", got)
	}

	if got := userStatus(t, db, 2); got != enum.UserStatusBanned {
		t.Fatalf("expected user 2 still banned, got %d", got)
	}

	if got := userStatus(t, db, 3); got != enum.UserStatusPermanentlyBanned {
		t.Fatalf("expected user 3 still permanently banned, got %d", got)
	}

	// 自动解封只标记原封禁记录，不补写 Active 记录。
	var records []po.UserBanRecord
	if err := db.Where("user_id = ?", 1).Find(&records).Error; err != nil {
		t.Fatalf("load ban records failed: %v", err)
	}

	if len(records) != 1 || records[0].Status != enum.UserStatusBanned || !records[0].IsUnbanned {
		t.Fatalf("unexpected released original record: %+v", records)
	}

	// 再跑一轮不得重复处理已解封记录。
	service.Run(context.Background())
	if err := db.Where("user_id = ?", 1).Find(&records).Error; err != nil {
		t.Fatalf("reload ban records failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected no duplicate records, got %d", len(records))
	}

	// 解封后必须 write-through Active，且只写到期用户。
	if len(recorder.writes) != 1 || recorder.writes[1] != enum.UserStatusActive {
		t.Fatalf("unexpected status cache writes: %v", recorder.writes)
	}
}

func TestAutoUnbanNoopWhenNothingExpired(t *testing.T) {
	db := openAutoUnbanTestDB(t)

	seedBanUser(t, db, 5, enum.UserStatusBanned, ptrTime(time.Now().Add(time.Hour)))

	recorder := newStatusWriteRecorder()
	service := app.NewAutoUnbanService(
		gormx.NewTxManager(db),
		NewBanMaintenanceRepository(db),
		recorder,
		zap.NewNop(),
	)

	service.Run(context.Background())

	if got := userStatus(t, db, 5); got != enum.UserStatusBanned {
		t.Fatalf("expected user 5 still banned, got %d", got)
	}

	if len(recorder.writes) != 0 {
		t.Fatalf("expected no status cache writes, got %v", recorder.writes)
	}
}

func seedBanUser(t *testing.T, db *gorm.DB, userID uint, status enum.UserStatus, endAt *time.Time) {
	t.Helper()

	if err := db.Exec(`INSERT INTO users (id, status) VALUES (?, ?)`, userID, status).Error; err != nil {
		t.Fatalf("seed user %d failed: %v", userID, err)
	}

	record := po.UserBanRecord{
		UserID:     userID,
		Status:     status,
		Reason:     "测试",
		Days:       1,
		StartAt:    time.Now().Add(-2 * time.Hour),
		EndAt:      endAt,
		OperatorID: 99,
	}

	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("seed ban record for user %d failed: %v", userID, err)
	}
}

func userStatus(t *testing.T, db *gorm.DB, userID uint) enum.UserStatus {
	t.Helper()

	var status enum.UserStatus
	if err := db.Table("users").Select("status").Where("id = ?", userID).Scan(&status).Error; err != nil {
		t.Fatalf("load user %d status failed: %v", userID, err)
	}

	return status
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
