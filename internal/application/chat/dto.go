// Package chat 编排私信发送、会话可见性与已读推进。
//
// chat_service.go 编排参与者、关注关系、发送限制和消息存储；规则由 domain/chat 表达。
// 消息写入与事件广播是两个完成点。WebSocket/Redis 通知用于实时展示，读取历史仍需经过查询端口。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package chat

import "time"

// MediaItem 消息媒体信息（json tag 与原 mongoEntity.ChatMessageMedia 逐字一致）.
type MediaItem struct {
	URL    string `json:"url"`
	Mime   string `json:"mime,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Size   int64  `json:"size,omitempty"`
}

// MessageItem 私信消息读模型（json tag 与原 mongoEntity.ChatMessage 逐字一致）.
//
// ID / 消息 ID 在原实现中是 primitive.ObjectID，其 JSON 形态即十六进制字符串，
// 这里用 string 承载以保持应用层不依赖 mongo 驱动，序列化结果完全相同。
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

// LastMessageItem 会话最后一条消息摘要读模型
// （json tag 与原 mongoEntity.ChatConversationLastMessage 逐字一致）.
type LastMessageItem struct {
	MsgID     string    `json:"msgId"`
	Type      string    `json:"type"`
	Preview   string    `json:"preview"`
	SenderID  uint      `json:"senderId"`
	CreatedAt time.Time `json:"createdAt"`
}

// ConversationItem 会话列表项读模型（json tag 与原 chat_service.ConversationItem 逐字一致）.
type ConversationItem struct {
	ID          string           `json:"id"`
	PeerID      uint             `json:"peerId"`
	Unread      int64            `json:"unread"`
	LastMessage *LastMessageItem `json:"lastMessage,omitempty"`
	UpdatedAt   int64            `json:"updatedAt"`
}

// MessageListResult 消息列表响应（json tag 与原 chat_service.MessageListResult 逐字一致）.
type MessageListResult struct {
	List       []MessageItem `json:"list"`
	NextBefore string        `json:"nextBefore,omitempty"`
	HasMore    bool          `json:"hasMore"`
}
