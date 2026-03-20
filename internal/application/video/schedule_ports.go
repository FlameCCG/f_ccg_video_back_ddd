package video

import (
	"context"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// ScheduleStore 读写 videos.publish_at；写操作复用视频状态变更的事务。
type ScheduleStore interface {
	// Available 报告持久化存储是否已配置，不依赖 Redis。
	Available() bool
	// Add 设置视频的定时发布时间。
	Add(ctx context.Context, videoID uint, when time.Time) error
	// Remove 清空视频的定时发布时间。
	Remove(ctx context.Context, videoID uint) error
	// PublishTime 读取定时发布时间；未定时或视频不存在时返回 nil。
	PublishTime(ctx context.Context, videoID uint) (*time.Time, error)
	// DueVideoIDs 读取本轮到期的视频 ID。
	DueVideoIDs(ctx context.Context, now time.Time, statuses []enum.VideoStatus) ([]uint, error)
}
