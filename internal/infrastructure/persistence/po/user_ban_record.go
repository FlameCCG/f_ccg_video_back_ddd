package po

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"time"
)

// UserBanRecord 用户封禁记录.
type UserBanRecord struct {
	Model
	UserID     uint            `gorm:"not null;index;comment:用户ID" json:"userId"`
	Status     enum.UserStatus `gorm:"not null;comment:状态 1-正常 2-封禁 3-永久封禁" json:"status"`
	Reason     string          `gorm:"size:500;comment:封禁原因" json:"reason"`
	Days       int             `gorm:"comment:封禁天数" json:"days"`
	StartAt    time.Time       `gorm:"comment:封禁开始时间" json:"startAt"`
	EndAt      *time.Time      `gorm:"comment:封禁结束时间" json:"endAt,omitempty"`
	OperatorID uint            `gorm:"comment:操作人ID" json:"operatorId"`
	IsUnbanned bool            `gorm:"not null;default:0;index;comment:是否已解封" json:"isUnbanned"`
}

// TableName 方法.
func (UserBanRecord) TableName() string {
	return "user_ban_records"
}
