package migration

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
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
	db := openFlagDBResourceIndexTestDB(t)

	if !db.Migrator().HasIndex(&po.VideoResource{}, videoResourceVariantIndexName) {
		t.Fatalf("expected %s to exist", videoResourceVariantIndexName)
	}

	if !db.Migrator().HasIndex(&po.VideoPartResource{}, videoPartResourceVariantIndexName) {
		t.Fatalf("expected %s to exist", videoPartResourceVariantIndexName)
	}

	if err := db.Create(&po.VideoResource{
		VideoID:        1,
		Resolution:     enum.Resolution720P,
		SourceFileName: "video.mp4",
		FileURL:        "video-url",
		FileSize:       100,
		Bitrate:        1000,
		Format:         testVideoFormat,
		Codec:          testVideoCodec,
	}).Error; err != nil {
		t.Fatalf("create video resource failed: %v", err)
	}

	if err := db.Create(&po.VideoResource{
		VideoID:        1,
		Resolution:     enum.Resolution720P,
		SourceFileName: "duplicate.mp4",
		FileURL:        "duplicate-url",
		FileSize:       200,
		Bitrate:        2000,
		Format:         testVideoFormat,
		Codec:          testVideoCodec,
	}).Error; err == nil {
		t.Fatal("expected duplicate video resource insert to fail")
	}

	if err := db.Create(&po.VideoPartResource{
		VideoPartID:    11,
		Resolution:     enum.Resolution360P,
		SourceFileName: "part.mp4",
		FileURL:        "part-url",
		FileSize:       100,
		Bitrate:        1000,
		Format:         testVideoFormat,
		Codec:          testVideoCodec,
	}).Error; err != nil {
		t.Fatalf("create video part resource failed: %v", err)
	}

	if err := db.Create(&po.VideoPartResource{
		VideoPartID:    11,
		Resolution:     enum.Resolution360P,
		SourceFileName: "duplicate-part.mp4",
		FileURL:        "duplicate-part-url",
		FileSize:       200,
		Bitrate:        2000,
		Format:         testVideoFormat,
		Codec:          testVideoCodec,
	}).Error; err == nil {
		t.Fatal("expected duplicate video part resource insert to fail")
	}
}

func openFlagDBResourceIndexTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "resource-index-test.db")), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite db failed: %v", err)
	}

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if err := db.AutoMigrate(&po.VideoResource{}, &po.VideoPartResource{}); err != nil {
		t.Fatalf("auto migrate resource tables failed: %v", err)
	}

	return db
}
