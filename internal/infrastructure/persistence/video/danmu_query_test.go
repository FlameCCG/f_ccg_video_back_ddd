package video_test

import (
	"context"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openDanmuQueryTestDB 只迁移弹幕表。
// 不复用 newFixture：它会因 video_like / video_coin 同名索引 idx_video_user 在 sqlite 上失败。
func openDanmuQueryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "danmu-query.db") +
		"?_pragma=synchronous(OFF)&_pragma=journal_mode(MEMORY)"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		TranslateError:         true,
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err, "open sqlite: %v", err)

	{
		err := db.AutoMigrate(&po.Danmu{})
		require.NoError(t, err, "migrate danmu: %v", err)
	}

	sqlDB, err := db.DB()
	require.NoError(t, err, "db.DB: %v", err)

	t.Cleanup(func() { _ = sqlDB.Close() })

	return db
}

func TestDanmuQueryMonthDates(t *testing.T) {
	t.Parallel()

	db := openDanmuQueryTestDB(t)
	query := infra.NewDanmuQuery(db)

	october21 := time.Date(2026, 10, 21, 15, 4, 0, 0, time.Local)

	rows := []po.Danmu{
		danmuAt(7, 0, "today", october21),
		danmuAt(7, 0, "today again", october21.Add(time.Hour)),
		danmuAt(7, 0, "three days ago", october21.AddDate(0, 0, -3)),
		danmuAt(7, 0, "next month", time.Date(2026, 11, 1, 0, 30, 0, 0, time.Local)),
		danmuAt(7, 0, "previous month", time.Date(2026, 9, 30, 23, 0, 0, 0, time.Local)),
		danmuAt(8, 0, "other video", october21),
		danmuAt(7, 3, "other part", october21),
	}
	{
		err := db.Session(&gorm.Session{SkipHooks: true}).Create(&rows).Error
		require.NoError(t, err, "create danmu: %v", err)
	}

	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)

	instants, err := query.MonthInstants(context.Background(), 7, 0, start, end)
	require.NoError(t, err)

	require.Len(t, instants, 3, "instants = %v", instants)

	require.True(t, instants[0].Equal(october21.AddDate(0, 0, -3)), "instants = %v", instants)
	require.True(t, instants[2].Equal(october21.Add(time.Hour)), "instants = %v", instants)
}

func danmuAt(videoID, partID uint, content string, createdAt time.Time) po.Danmu {
	return po.Danmu{
		Model:       po.Model{CreatedAt: createdAt},
		UserID:      1,
		VideoID:     videoID,
		VideoPartID: partID,
		Content:     content,
		TimeOffset:  1000,
		Color:       "#FFFFFF",
		FontSize:    25,
	}
}
