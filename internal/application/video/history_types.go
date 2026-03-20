package video

import (
	"time"
)

// HistoryItem 观看历史列表项。
type HistoryItem struct {
	VideoID   uint      `json:"videoId"`
	Title     string    `json:"title"`
	Cover     string    `json:"cover"`
	Duration  int       `json:"duration"`
	Progress  int       `json:"progress"`
	CreatedAt time.Time `json:"createdAt"`
	AuthorID  uint      `json:"authorId"`
	Author    string    `json:"author"`
}
