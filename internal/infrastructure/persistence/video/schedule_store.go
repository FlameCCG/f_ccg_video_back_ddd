package video

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	domain "f_ccg_video_back_ddd/internal/domain/video"

	"gorm.io/gorm"
)

// ScheduleStore 只读写视频自身的发布时间，与状态变更复用同一事务。
type ScheduleStore struct {
	db *gorm.DB
}

var _ app.ScheduleStore = (*ScheduleStore)(nil)

// NewScheduleStore 将发布计划端口绑定到共享 MySQL 连接；实际操作仍经 gormx 选择事务。
func NewScheduleStore(db *gorm.DB) *ScheduleStore { return &ScheduleStore{db: db} }

// Available 仅检查句柄是否配置，不主动探测数据库连通性。
func (s *ScheduleStore) Available() bool { return s != nil && s.db != nil }

// Add 保存计划时刻并设置定时意图；调用方需要自行保证视频存在、业务状态正确及事务范围。
func (s *ScheduleStore) Add(ctx context.Context, videoID uint, when time.Time) error {
	if videoID == 0 {
		return domain.ErrVideoIDRequired
	}

	return gormx.DB(ctx, s.db).Model(&po.Video{}).Where("id = ?", videoID).
		UpdateColumns(map[string]any{"publish_at": when, "is_scheduled": true}).Error
}

// Remove 清空 publish_at，不修改 is_scheduled；历史定时标记由视频状态/改稿用例另行维护。
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

// PublishTime 读取计划时刻；无记录和未设置计划都返回 nil 时间，调用方另查视频状态。
func (s *ScheduleStore) PublishTime(ctx context.Context, videoID uint) (*time.Time, error) {
	var row po.Video

	err := gormx.DB(ctx, s.db).Select("publish_at").Take(&row, "id = ?", videoID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return row.PublishAt, err
}

// DueVideoIDs 按发布时间和 ID 稳定排序，最多返回 100 个候选，不在此领取或锁定任务。
// 并发调度的最终裁定在 Scheduler.ProcessDueScheduledVideo 的行锁事务中。
func (s *ScheduleStore) DueVideoIDs(ctx context.Context, now time.Time, statuses []enum.VideoStatus) ([]uint, error) {
	var ids []uint

	err := gormx.DB(ctx, s.db).Model(&po.Video{}).Where("publish_at <= ?", now).
		Where("status IN ?", statuses).
		Order("publish_at ASC, id ASC").Limit(100).Pluck("id", &ids).Error

	return ids, err
}
