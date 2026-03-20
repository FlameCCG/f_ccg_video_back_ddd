package video

import (
	"time"
)

// DanmuDTO 弹幕响应体（同时是 Redis 最近弹幕缓存格式）。
type DanmuDTO struct {
	ID          uint      `json:"id"`
	VideoID     uint      `json:"videoId"`
	VideoPartID uint      `json:"videoPartId"`
	UserID      uint      `json:"userId"`
	Content     string    `json:"content"`
	TimeOffset  int       `json:"timeOffset"`
	Color       string    `json:"color"`
	FontSize    int8      `json:"fontSize"`
	Position    int8      `json:"position"`
	LikeCount   int64     `json:"likeCount"`
	IsLiked     bool      `json:"isLiked"`
	CreatedAt   time.Time `json:"createdAt"`
}

// DanmuRoomItem 房间弹幕列表项。
type DanmuRoomItem struct {
	ID          uint      `json:"id"`
	VideoID     uint      `json:"videoId"`
	VideoPartID uint      `json:"videoPartId"`
	UserID      uint      `json:"userId"`
	Content     string    `json:"content"`
	TimeOffset  int       `json:"timeOffset"`
	Color       string    `json:"color"`
	FontSize    int8      `json:"fontSize"`
	Position    int8      `json:"position"`
	LikeCount   int64     `json:"likeCount"`
	IsLiked     bool      `json:"isLiked"`
	CreatedAt   time.Time `json:"createdAt"`
}

// DanmuLikeResult 弹幕点赞结果。
type DanmuLikeResult struct {
	LikeCount int64 `json:"likeCount"`
	IsLiked   bool  `json:"isLiked"`
}

// AdminDanmuItem 管理端弹幕列表项。
type AdminDanmuItem struct {
	ID          uint      `json:"id"`
	UserID      uint      `json:"userId"`
	Username    string    `json:"username"`
	Avatar      string    `json:"avatar"`
	VideoID     uint      `json:"videoId"`
	VideoTitle  string    `json:"videoTitle,omitempty"`
	VideoCover  string    `json:"videoCover,omitempty"`
	VideoPartID uint      `json:"videoPartId"`
	Content     string    `json:"content"`
	TimeOffset  int       `json:"timeOffset"`
	Color       string    `json:"color"`
	FontSize    int8      `json:"fontSize"`
	Position    int8      `json:"position"`
	LikeCount   int64     `json:"likeCount"`
	CreatedAt   time.Time `json:"createdAt"`
}

// CreatorDanmuItem 创作中心弹幕列表项。
type CreatorDanmuItem struct {
	ID          uint      `json:"id"`
	UserID      uint      `json:"userId"`
	Username    string    `json:"username"`
	Avatar      string    `json:"avatar"`
	VideoID     uint      `json:"videoId"`
	VideoTitle  string    `json:"videoTitle,omitempty"`
	VideoCover  string    `json:"videoCover,omitempty"`
	VideoPartID uint      `json:"videoPartId"`
	Content     string    `json:"content"`
	TimeOffset  int       `json:"timeOffset"`
	Color       string    `json:"color"`
	FontSize    int8      `json:"fontSize"`
	Position    int8      `json:"position"`
	LikeCount   int64     `json:"likeCount"`
	CreatedAt   time.Time `json:"createdAt"`
}

// SendDanmuParams 发送弹幕参数。
type SendDanmuParams struct {
	VideoID     uint
	VideoPartID uint
	Content     string
	TimeOffset  int
	Color       string
	FontSize    int8
	Position    int8
}

// DanmuHistoryParams 历史弹幕查询参数。
type DanmuHistoryParams struct {
	VideoID     uint
	VideoPartID uint
	UserID      uint
	Start       int
	End         int
	Limit       int
}

// DanmuMonthDates 某月内有弹幕的发送时刻。
//
// Times 是库内 Asia/Shanghai 的绝对时间。浏览器再用本地时区把它折成日历日。
type DanmuMonthDates struct {
	Month string      `json:"month"`
	Times []time.Time `json:"times"`
}

// DanmuRoomListParams 房间弹幕列表查询参数。
type DanmuRoomListParams struct {
	VideoID      uint
	VideoPartID  uint
	Page         int
	PageSize     int
	Date         string // YYYY-MM-DD, optional
	UserID       uint   // optional (0 means anonymous)
	Start        int    // 播放时间起点（毫秒，含）
	End          int    // 播放时间终点（毫秒，不含）
	UseTimeRange bool   // true 时按 [Start, End) 查询播放分段
}

// DanmuRoomQueryParams 房间弹幕的读侧查询参数（已完成归一化）。
type DanmuRoomQueryParams struct {
	VideoID      uint
	VideoPartID  uint
	Page         int
	PageSize     int
	UseTimeRange bool
	Start        int
	End          int
	// DateStart/DateEnd 为 Date 解析结果；HasDate 为 false 时忽略。
	HasDate   bool
	DateStart time.Time
	DateEnd   time.Time
}

// AdminDanmuListParams 管理端弹幕列表查询参数。
type AdminDanmuListParams struct {
	UserID   uint
	Username string
	Keyword  string
	Page     int
	PageSize int
}
