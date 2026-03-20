package chat

import "time"

// MediaItem 消息媒体信息.
type MediaItem struct {
	URL    string `json:"url"`
	Mime   string `json:"mime,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Size   int64  `json:"size,omitempty"`
}

// MessageItem 私信消息读模型。
//
// 消息 ID 使用十六进制字符串，避免应用层依赖 Mongo 驱动。
type MessageItem struct {
	ID             string     `json:"id"`
	ConversationID string     `json:"conversationId"`
	SenderID       uint       `json:"senderId"`
	ReceiverID     uint       `json:"receiverId"`
	Type           string     `json:"type"`
	Text           string     `json:"text,omitempty"`
	Emoji          string     `json:"emoji,omitempty"`
	Media          *MediaItem `json:"media,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// LastMessageItem 会话最后一条消息摘要读模型。
type LastMessageItem struct {
	MsgID     string    `json:"msgId"`
	Type      string    `json:"type"`
	Preview   string    `json:"preview"`
	SenderID  uint      `json:"senderId"`
	CreatedAt time.Time `json:"createdAt"`
}

// MessageListResult 消息列表响应.
type MessageListResult struct {
	List       []MessageItem `json:"list"`
	NextBefore string        `json:"nextBefore,omitempty"`
	HasMore    bool          `json:"hasMore"`
}
