package notification

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
