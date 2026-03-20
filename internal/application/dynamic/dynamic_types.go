package dynamic

import (
	"time"
)

// DynamicItem 动态列表项（读模型）.
type DynamicItem struct {
	ID        uint       `json:"id"`
	UserID    uint       `json:"userId"`
	Username  string     `json:"username"`
	Avatar    string     `json:"avatar"`
	Content   string     `json:"content"`
	ImageURL  string     `json:"imageUrl"`
	IsPinned  bool       `json:"isPinned"`
	PinnedAt  *time.Time `json:"pinnedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}
