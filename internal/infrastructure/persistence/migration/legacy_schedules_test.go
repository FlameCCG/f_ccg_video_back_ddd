package migration

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
	"time"
)

func TestLegacyScheduleBackfillIsConsumedAndDoesNotRestoreCancelledPlans(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	t.Cleanup(func() { _ = conn.Close() })
	if err = db.AutoMigrate(&po.Video{}); err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("CREATE TABLE video_publish_schedules(video_id integer PRIMARY KEY,publish_at datetime)").Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	for _, v := range []po.Video{{Model: po.Model{ID: 1}, Status: enum.VideoStatusScheduled}, {Model: po.Model{ID: 2}, Status: enum.VideoStatusDeleted}, {Model: po.Model{ID: 3}, Status: enum.VideoStatusReview}} {
		if err = db.Create(&v).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []uint{1, 2, 3} {
		if err = db.Exec("INSERT INTO video_publish_schedules VALUES (?,?)", id, now).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = migrateLegacySchedules(db); err != nil {
		t.Fatal(err)
	}
	var v po.Video
	if err = db.First(&v, 1).Error; err != nil {
		t.Fatal(err)
	}
	if v.PublishAt == nil || !v.PublishAt.Equal(now) || !v.IsScheduled {
		t.Fatalf("not migrated: %+v", v)
	}
	v = po.Video{}
	db.First(&v, 2)
	if v.PublishAt != nil {
		t.Fatal("deleted video gained a schedule")
	}
	if err = db.Model(&po.Video{}).Where("id=1").UpdateColumn("publish_at", nil).Error; err != nil {
		t.Fatal(err)
	}
	if err = migrateLegacySchedules(db); err != nil {
		t.Fatal(err)
	}
	v = po.Video{}
	db.First(&v, 1)
	if v.PublishAt != nil {
		t.Fatal("migration restored cancelled plan")
	}
	var n int64
	db.Table("video_publish_schedules").Count(&n)
	if n != 0 {
		t.Fatal("legacy rows not consumed")
	}
}
