package notification

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// NotificationItem 通知列表项（读模型）.
type NotificationItem struct {
	ID               uint      `json:"id"`
	Type             enum.Type `json:"type"`
	ReceiverID       int       `json:"receiverID"`
	ActionUserID     uint      `json:"actionUserID"`
	ActionUserAvatar string    `json:"actionUserAvatar"`
	ActionUserName   string    `json:"actionUserName"`
	Title            string    `json:"title"`
	Content          string    `json:"content"`
	VideoID          uint      `json:"videoID"`
	VideoTitle       string    `json:"videoTitle"`
	Link             string    `json:"link"`
	CommentID        uint      `json:"commentID"`
	DynamicID        uint      `json:"dynamicID"`
	IsRead           bool      `json:"isRead"`
	CreatedAt        time.Time `json:"createdAt"`
}
