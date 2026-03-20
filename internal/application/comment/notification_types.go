package comment

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// Notice 通知写入载荷，作为跨上下文端口 Notifier 的入参。
type Notice struct {
	Type             enum.Type
	ReceiverID       int
	ActionUserID     uint
	ActionUserAvatar string
	ActionUserName   string
	Title            string
	Content          string
	Link             string
	VideoID          uint
	VideoTitle       string
	CommentID        uint
}
