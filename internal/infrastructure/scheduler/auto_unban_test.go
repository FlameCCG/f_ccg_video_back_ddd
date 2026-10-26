package scheduler

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/platform"

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
	require.NoError(t, err, "open sqlite failed: %v", err)

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
		{
			err := db.Exec(stmt).Error
			require.NoError(t, err, "create test tables failed: %v", err)
		}
	}

	sqlDB, err := db.DB()
	require.NoError(t, err, "get sql db failed: %v", err)

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}

func TestAutoUnbanReleasesOnlyExpiredBans(t *testing.T) {
	t.Parallel()

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

	{
		got := userStatus(t, db, 1)
		require.Equal(t, enum.UserStatusActive, got, "expected user 1 active, got %d", got)
	}

	{
		got := userStatus(t, db, 2)
		require.EqualValues(t, enum.UserStatusBanned, got, "expected user 2 still banned, got %d", got)
	}

	{
		got := userStatus(t, db, 3)
		require.EqualValues(t, enum.UserStatusPermanentlyBanned, got, "expected user 3 still permanently banned, got %d", got)
	}

	// 自动解封只标记原封禁记录，不补写 Active 记录。
	var records []po.UserBanRecord
	{
		err := db.Where("user_id = ?", 1).Find(&records).Error
		require.NoError(t, err, "load ban records failed: %v", err)
	}

	require.Len(t, records, 1, "unexpected released original record: %+v", records)
	require.EqualValues(t, enum.UserStatusBanned, records[0].Status, "unexpected released original record: %+v", records)
	require.True(t, records[0].IsUnbanned, "unexpected released original record: %+v", records)

	// 再跑一轮不得重复处理已解封记录。
	service.Run(context.Background())

	{
		err := db.Where("user_id = ?", 1).Find(&records).Error
		require.NoError(t, err, "reload ban records failed: %v", err)
	}

	require.Len(t, records, 1, "expected no duplicate records, got %d", len(records))

	// 解封后必须 write-through Active，且只写到期用户。
	require.Len(t, recorder.writes, 1, "unexpected status cache writes: %v", recorder.writes)
	require.Equal(t, enum.UserStatusActive, recorder.writes[1], "unexpected status cache writes: %v", recorder.writes)
}

func TestAutoUnbanNoopWhenNothingExpired(t *testing.T) {
	t.Parallel()

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

	{
		got := userStatus(t, db, 5)
		require.EqualValues(t, enum.UserStatusBanned, got, "expected user 5 still banned, got %d", got)
	}

	require.Empty(t, recorder.writes, "expected no status cache writes, got %v", recorder.writes)
}

func seedBanUser(t *testing.T, db *gorm.DB, userID uint, status enum.UserStatus, endAt *time.Time) {
	t.Helper()

	{
		err := db.Exec(`INSERT INTO users (id, status) VALUES (?, ?)`, userID, status).Error
		require.NoError(t, err, "seed user %d failed: %v", userID, err)
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

	{
		err := db.Create(&record).Error
		require.NoError(t, err, "seed ban record for user %d failed: %v", userID, err)
	}
}

func userStatus(t *testing.T, db *gorm.DB, userID uint) enum.UserStatus {
	t.Helper()

	var status enum.UserStatus
	{
		err := db.Table("users").Select("status").Where("id = ?", userID).Scan(&status).Error
		require.NoError(t, err, "load user %d status failed: %v", userID, err)
	}

	return status
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
