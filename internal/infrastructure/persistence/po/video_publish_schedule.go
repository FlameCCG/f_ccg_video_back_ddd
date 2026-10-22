package po

import "time"

// VideoPublishSchedule 与视频写入共用事务。空 PublishAt 是取消标记，
// 防止尚未清理的旧 Redis 任务在恢复时覆盖取消或改期结果。
type VideoPublishSchedule struct {
	VideoID   uint       `gorm:"primaryKey;autoIncrement:false"`
	PublishAt *time.Time `gorm:"index:idx_video_publish_due"`
}

func (VideoPublishSchedule) TableName() string { return "video_publish_schedules" }
