package video

import (
	"time"
)

// FeedItem 首页/热门/排行/推荐的列表项。
type FeedItem struct {
	ID         uint            `json:"id"`
	Title      string          `json:"title"`
	Cover      string          `json:"cover"`
	Duration   int             `json:"duration"`
	Views      int64           `json:"views"`
	DanmuCount int64           `json:"danmuCount"`
	Author     FeedAuthorBrief `json:"author"`
	CreatedAt  time.Time       `json:"createdAt"`
}

// FeedAuthorBrief 列表项作者摘要。
type FeedAuthorBrief struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}
