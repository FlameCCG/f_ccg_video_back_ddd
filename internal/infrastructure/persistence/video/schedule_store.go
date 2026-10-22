package video

import (
	"context"
	"errors"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// ScheduleStore 只读写视频自身的发布时间，与状态变更复用同一事务。
type ScheduleStore struct {
	db *gorm.DB
}

var _ app.ScheduleStore = (*ScheduleStore)(nil)

func NewScheduleStore(db *gorm.DB) *ScheduleStore { return &ScheduleStore{db: db} }

func (s *ScheduleStore) Available() bool { return s != nil && s.db != nil }

func (s *ScheduleStore) Add(ctx context.Context, videoID uint, when time.Time) error {
	if videoID == 0 {
		return domain.ErrVideoIDRequired
	}
	return gormx.DB(ctx, s.db).Model(&po.Video{}).Where("id = ?", videoID).
		UpdateColumns(map[string]any{"publish_at": when, "is_scheduled": true}).Error
}

func (s *ScheduleStore) Remove(ctx context.Context, videoID uint) error {
	return s.save(ctx, videoID, nil)
}

func (s *ScheduleStore) save(ctx context.Context, videoID uint, when *time.Time) error {
	if videoID == 0 {
		return domain.ErrVideoIDRequired
	}
	return gormx.DB(ctx, s.db).Model(&po.Video{}).Where("id = ?", videoID).
		UpdateColumn("publish_at", when).Error
}

func (s *ScheduleStore) PublishTime(ctx context.Context, videoID uint) (*time.Time, error) {
	var row po.Video
	err := gormx.DB(ctx, s.db).Select("publish_at").Take(&row, "id = ?", videoID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return row.PublishAt, err
}

func (s *ScheduleStore) DueVideoIDs(ctx context.Context, now time.Time, statuses []enum.VideoStatus) ([]uint, error) {
	var ids []uint
	err := gormx.DB(ctx, s.db).Model(&po.Video{}).Where("publish_at <= ?", now).
		Where("status IN ?", statuses).
		Order("publish_at ASC, id ASC").Limit(100).Pluck("id", &ids).Error
	return ids, err
}
