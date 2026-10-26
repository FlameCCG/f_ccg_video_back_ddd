package migration

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// migrateLegacySchedules runs with writers stopped during the schema upgrade.
// Consuming legacy rows in the same transaction makes retries safe and prevents
// a later migration from restoring a subsequently cancelled schedule.
func migrateLegacySchedules(db *gorm.DB) error {
	if !db.Migrator().HasTable("video_publish_schedules") {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var rows []struct {
			VideoID   uint
			PublishAt *time.Time
		}
		if err := tx.Table("video_publish_schedules").Find(&rows).Error; err != nil {
			return err
		}

		for _, row := range rows {
			if row.PublishAt != nil {
				err := tx.Model(&po.Video{}).Where("id = ? AND publish_at IS NULL AND status IN ?", row.VideoID, []enum.VideoStatus{enum.VideoStatusScheduled, enum.VideoStatusReview}).UpdateColumns(map[string]any{"publish_at": row.PublishAt, "is_scheduled": true}).Error
				if err != nil {
					return fmt.Errorf("migrate legacy schedule %d: %w", row.VideoID, err)
				}
			}

			if err := tx.Table("video_publish_schedules").Where("video_id = ?", row.VideoID).Delete(nil).Error; err != nil {
				return err
			}
		}

		return nil
	})
}
