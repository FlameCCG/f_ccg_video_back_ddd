package po

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"time"

	"gorm.io/datatypes"
)

// VideoReport 视频举报记录.
type VideoReport struct {
	Model
	ReporterUsername string            `gorm:"size:50;not null;index;comment:举报人用户名" json:"reporterUsername"`
	VideoID          uint              `gorm:"not null;index;comment:视频ID" json:"videoId"`
	Reason           string            `gorm:"size:200;comment:举报原因" json:"reason"`
	Detail           string            `gorm:"type:text;comment:举报详情" json:"detail"`
	ImageUrls        datatypes.JSON    `gorm:"type:json;comment:图片列表JSON" json:"imageUrls"`
	Status           enum.ReportStatus `gorm:"default:1;index;comment:处理状态 1-待处理 2-已处理 3-已驳回" json:"status"`
	HandleNote       string            `gorm:"size:500;comment:处理说明" json:"handleNote"`
	HandledBy        uint              `gorm:"comment:处理人ID" json:"handledBy"`
	HandledAt        *time.Time        `gorm:"comment:处理时间" json:"handledAt"`
}

// TableName 显式绑定表名 video_reports，保持迁移与仓储 SQL 使用同一张表。
func (VideoReport) TableName() string {
	return "video_reports"
}
