package chat

// ConversationItem 会话列表项读模型.
type ConversationItem struct {
	ID          string           `json:"id"`
	PeerID      uint             `json:"peerId"`
	Unread      int64            `json:"unread"`
	LastMessage *LastMessageItem `json:"lastMessage,omitempty"`
	UpdatedAt   int64            `json:"updatedAt"`
}
