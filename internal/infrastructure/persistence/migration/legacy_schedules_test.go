package migration

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestLegacyScheduleBackfillIsConsumedAndDoesNotRestoreCancelledPlans(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy.db")), &gorm.Config{})
	require.NoError(t, err)

	conn, _ := db.DB()

	t.Cleanup(func() { _ = conn.Close() })

	{
		err = db.AutoMigrate(&po.Video{})
		require.NoError(t, err)
	}

	{
		err = db.Exec("CREATE TABLE video_publish_schedules(video_id integer PRIMARY KEY,publish_at datetime)").Error
		require.NoError(t, err)
	}

	now := time.Now().UTC().Truncate(time.Second)

	for _, v := range []po.Video{{Model: po.Model{ID: 1}, Status: enum.VideoStatusScheduled}, {Model: po.Model{ID: 2}, Status: enum.VideoStatusDeleted}, {Model: po.Model{ID: 3}, Status: enum.VideoStatusReview}} {
		{
			err = db.Create(&v).Error
			require.NoError(t, err)
		}
	}

	for _, id := range []uint{1, 2, 3} {
		{
			err = db.Exec("INSERT INTO video_publish_schedules VALUES (?,?)", id, now).Error
			require.NoError(t, err)
		}
	}

	{
		err = migrateLegacySchedules(db)
		require.NoError(t, err)
	}

	var v po.Video
	{
		err = db.First(&v, 1).Error
		require.NoError(t, err)
	}

	require.NotNil(t, v.PublishAt, "not migrated: %+v", v)
	require.True(t, v.PublishAt.Equal(now), "not migrated: %+v", v)
	require.True(t, v.IsScheduled, "not migrated: %+v", v)

	v = po.Video{}
	db.First(&v, 2)

	require.Nil(t, v.PublishAt, "deleted video gained a schedule")

	{
		err = db.Model(&po.Video{}).Where("id=1").UpdateColumn("publish_at", nil).Error
		require.NoError(t, err)
	}

	{
		err = migrateLegacySchedules(db)
		require.NoError(t, err)
	}

	v = po.Video{}
	db.First(&v, 1)

	require.Nil(t, v.PublishAt, "migration restored cancelled plan")

	var n int64
	db.Table("video_publish_schedules").Count(&n)

	require.EqualValues(t, 0, n, "legacy rows not consumed")
}
