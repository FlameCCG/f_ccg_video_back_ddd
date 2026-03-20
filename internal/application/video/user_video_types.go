package video

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// UserVideoItem 用户投稿列表项。
type UserVideoItem struct {
	ID              uint      `json:"id"`
	Title           string    `json:"title"`
	Cover           string    `json:"cover"`
	Views           int64     `json:"views"`
	DanmuCount      int64     `json:"danmuCount"`
	Duration        int       `json:"duration"`
	Progress        int       `json:"progress,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	Status          int       `json:"status"`
	StatusText      string    `json:"statusText"`
	AuditStatus     int       `json:"auditStatus,omitempty"`
	AuditStatusText string    `json:"auditStatusText,omitempty"`
	AuthorID        uint      `json:"authorId,omitempty"`
	AuthorName      string    `json:"authorName,omitempty"`
}

// UserVideoRow 用户投稿列表的原始行（读侧投影，未叠加 Redis 计数与进度）。
type UserVideoRow struct {
	ID             uint
	Title          string
	Cover          string
	Views          int64
	Likes          int64
	FavoriteCount  int64
	DanmuCount     int64
	Duration       int
	CreatedAt      time.Time
	Status         enum.VideoStatus
	AuthorUsername string
	AuthorID       uint
	AuthorName     string
}

// AuthorVideoStatRow 作者作品统计聚合的原始行。
type AuthorVideoStatRow struct {
	ID    uint
	Views int64
	Likes int64
}

// UserVideoStats 聚合用户作品曝光/互动数据。
type UserVideoStats struct {
	TotalLikes int64 `json:"totalLikes"`
	TotalViews int64 `json:"totalViews"`
}
