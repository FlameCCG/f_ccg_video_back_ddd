package po

import "time"

// VideoPlayHistory 播放历史记录.
type VideoPlayHistory struct {
	UserID    uint      `gorm:"primaryKey;comment:用户ID"`
	VideoID   uint      `gorm:"primaryKey;comment:视频ID"`
	Progress  int       `gorm:"default:0;comment:播放进度(秒)" json:"progress"`
	Duration  int       `gorm:"default:0;comment:视频总时长(秒)" json:"duration"`
	CreatedAt time.Time `gorm:"not null" json:"-"`
	// 关联关系
	User  User  `gorm:"foreignKey:UserID" json:"userEntity,omitempty"`
	Video Video `gorm:"foreignKey:VideoID" json:"video,omitempty"`
}

// TableName 显式绑定表名 video_play_histories，保持迁移与仓储 SQL 使用同一张表。
func (VideoPlayHistory) TableName() string {
	return "video_play_histories"
}
