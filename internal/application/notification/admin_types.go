package notification

import "f_ccg_video_back_ddd/internal/domain/shared/enum"

// AdminNotification 管理端系统通知响应，JSON 不输出时间字段，保留零值，不使用 omitempty。
type AdminNotification struct {
	ID               uint      `json:"id"`
	Type             enum.Type `json:"type"`
	ReceiverID       int       `json:"receiverID"`
	ActionUserID     uint      `json:"actionUserID"`
	ActionUserAvatar string    `json:"actionUserAvatar"`
	ActionUserName   string    `json:"actionUserName"`
	Title            string    `json:"title"`
	Content          string    `json:"content"`
	Link             string    `json:"link"`
	VideoID          uint      `json:"videoID"`
	VideoTitle       string    `json:"videoTitle"`
	DynamicID        uint      `json:"dynamicID"`
	CommentID        uint      `json:"commentID"`
}

// CreateSystemNoticeCommand 管理端发布系统通知的入参。
type CreateSystemNoticeCommand struct {
	ActorID    uint
	Title      string
	Content    string
	ReceiverID int
	VideoID    uint
	VideoTitle string
	Link       string
}

// UpdateSystemNoticeCommand 管理端更新系统通知的入参。
type UpdateSystemNoticeCommand struct {
	ID         uint
	Title      string
	Content    string
	ReceiverID int
	VideoID    uint
	VideoTitle string
	Link       string
}
