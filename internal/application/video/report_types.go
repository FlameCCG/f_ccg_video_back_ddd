package video

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// AdminVideoReportItem 管理端视频举报列表项。
type AdminVideoReportItem struct {
	ID               uint       `json:"id"`
	ReporterUsername string     `json:"reporterUsername"`
	VideoID          uint       `json:"videoId"`
	VideoTitle       string     `json:"videoTitle"`
	VideoCover       string     `json:"videoCover"`
	Reason           string     `json:"reason"`
	Detail           string     `json:"detail"`
	ImageUrls        []string   `json:"imageUrls"`
	Status           int        `json:"status"`
	HandleNote       string     `json:"handleNote"`
	HandledBy        uint       `json:"handledBy"`
	HandledAt        *time.Time `json:"handledAt"`
	CreatedAt        time.Time  `json:"createdAt"`
}

// AdminDanmuReportItem 管理端弹幕举报列表项。
type AdminDanmuReportItem struct {
	ID               uint       `json:"id"`
	ReporterUsername string     `json:"reporterUsername"`
	VideoID          uint       `json:"videoId"`
	VideoTitle       string     `json:"videoTitle"`
	VideoCover       string     `json:"videoCover"`
	DanmuID          uint       `json:"danmuId"`
	DanmuContent     string     `json:"danmuContent"`
	Reason           string     `json:"reason"`
	Detail           string     `json:"detail"`
	Status           int        `json:"status"`
	HandleNote       string     `json:"handleNote"`
	HandledBy        uint       `json:"handledBy"`
	HandledAt        *time.Time `json:"handledAt"`
	CreatedAt        time.Time  `json:"createdAt"`
}

// VideoReportDTO 视频举报响应体（形态等同 po.VideoReport 序列化结果）。
type VideoReportDTO struct {
	ID               uint              `json:"id"`
	ReporterUsername string            `json:"reporterUsername"`
	VideoID          uint              `json:"videoId"`
	Reason           string            `json:"reason"`
	Detail           string            `json:"detail"`
	ImageUrls        []string          `json:"imageUrls"`
	Status           enum.ReportStatus `json:"status"`
	HandleNote       string            `json:"handleNote"`
	HandledBy        uint              `json:"handledBy"`
	HandledAt        *time.Time        `json:"handledAt"`
}

// DanmuReportDTO 弹幕举报响应体（形态等同 po.DanmuReport 序列化结果）。
type DanmuReportDTO struct {
	ID               uint              `json:"id"`
	ReporterUsername string            `json:"reporterUsername"`
	VideoID          uint              `json:"videoId"`
	DanmuID          uint              `json:"danmuId"`
	Reason           string            `json:"reason"`
	Detail           string            `json:"detail"`
	Status           enum.ReportStatus `json:"status"`
	HandleNote       string            `json:"handleNote"`
	HandledBy        uint              `json:"handledBy"`
	HandledAt        *time.Time        `json:"handledAt"`
}
