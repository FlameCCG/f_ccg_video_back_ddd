package migration

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	testVideoFormat = "mp4"
	testVideoCodec  = "h264"
)

const (
	videoResourceVariantIndexName     = "uk_video_resources_variant"
	videoPartResourceVariantIndexName = "uk_video_part_resources_variant"
)

func TestVideoResourceUniqueIndexesFromAutoMigrate(t *testing.T) {
	t.Parallel()

	db := openFlagDBResourceIndexTestDB(t)

	require.True(t, db.Migrator().HasIndex(&po.VideoResource{}, videoResourceVariantIndexName), "expected %s to exist", videoResourceVariantIndexName)

	require.True(t, db.Migrator().HasIndex(&po.VideoPartResource{}, videoPartResourceVariantIndexName), "expected %s to exist", videoPartResourceVariantIndexName)

	{
		err := db.Create(&po.VideoResource{
			VideoID:        1,
			Resolution:     enum.Resolution720P,
			SourceFileName: "video.mp4",
			FileURL:        "video-url",
			FileSize:       100,
			Bitrate:        1000,
			Format:         testVideoFormat,
			Codec:          testVideoCodec,
		}).Error
		require.NoError(t, err, "create video resource failed: %v", err)
	}

	{
		err := db.Create(&po.VideoResource{
			VideoID:        1,
			Resolution:     enum.Resolution720P,
			SourceFileName: "duplicate.mp4",
			FileURL:        "duplicate-url",
			FileSize:       200,
			Bitrate:        2000,
			Format:         testVideoFormat,
			Codec:          testVideoCodec,
		}).Error
		require.Error(t, err, "expected duplicate video resource insert to fail")
	}

	{
		err := db.Create(&po.VideoPartResource{
			VideoPartID:    11,
			Resolution:     enum.Resolution360P,
			SourceFileName: "part.mp4",
			FileURL:        "part-url",
			FileSize:       100,
			Bitrate:        1000,
			Format:         testVideoFormat,
			Codec:          testVideoCodec,
		}).Error
		require.NoError(t, err, "create video part resource failed: %v", err)
	}

	{
		err := db.Create(&po.VideoPartResource{
			VideoPartID:    11,
			Resolution:     enum.Resolution360P,
			SourceFileName: "duplicate-part.mp4",
			FileURL:        "duplicate-part-url",
			FileSize:       200,
			Bitrate:        2000,
			Format:         testVideoFormat,
			Codec:          testVideoCodec,
		}).Error
		require.Error(t, err, "expected duplicate video part resource insert to fail")
	}
}

func openFlagDBResourceIndexTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "resource-index-test.db")), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err, "open sqlite failed: %v", err)

	sqlDB, err := db.DB()
	require.NoError(t, err, "get sqlite db failed: %v", err)

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	{
		err := db.AutoMigrate(&po.VideoResource{}, &po.VideoPartResource{})
		require.NoError(t, err, "auto migrate resource tables failed: %v", err)
	}

	return db
}
