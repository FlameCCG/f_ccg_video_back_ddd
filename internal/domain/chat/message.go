package chat

import (
	"time"
	"unicode/utf8"
)

// 消息类型（与 Mongo 文档 type 字段取值一致，属于外部契约）。
const (
	// MessageTypeText 文本消息。
	MessageTypeText = "text"
	// MessageTypeImage 图片消息。
	MessageTypeImage = "image"
	// MessageTypeEmoji emoji 消息。
	MessageTypeEmoji = "emoji"
	// MessageTypeSticker 表情包消息。
	MessageTypeSticker = "sticker"
)

const (
	// maxTextRunes 文本消息最大字数。
	maxTextRunes = 1000
	// maxPreviewRunes 会话最后一条消息预览最大字数。
	maxPreviewRunes = 50
)

// Media 消息附带的媒体载荷。字段保持公开，供构造参数与快照使用。
type Media struct {
	URL    string
	Mime   string
	Width  int
	Height int
	Size   int64
}

func (m *Media) clone() *Media {
	if m == nil {
		return nil
	}

	copied := *m

	return &copied
}

// MessageSnapshot 是消息的持久化快照。字段公开，恢复时不重新执行创建校验。
type MessageSnapshot struct {
	ID             string
	ConversationID string
	SenderID       uint
	ReceiverID     uint
	Type           string
	Text           string
	Emoji          string
	Media          *Media
	CreatedAt      time.Time
}

// Message 私信消息实体。
//
// ID 为 Mongo ObjectID 的十六进制表示。仓储落库后通过 RestoreMessage 返回带 ID 的副本，
// 领域层不依赖具体存储的 ID 类型。
type Message struct {
	id             string
	conversationID string
	senderID       uint
	receiverID     uint
	messageType    string
	text           string
	emoji          string
	media          *Media
	createdAt      time.Time
}

// NewMessage 在构造边界统一检查参与者和内容，并生成确定性的会话身份。
func NewMessage(senderID, receiverID uint, msgType, text, emoji string, media *Media, now time.Time) (*Message, error) {
	msg := &Message{
		senderID:    senderID,
		receiverID:  receiverID,
		messageType: msgType,
		text:        text,
		emoji:       emoji,
		media:       media.clone(),
		createdAt:   now,
	}
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	msg.conversationID = (Conversation{}).BuildConversationID(senderID, receiverID)

	return msg, nil
}

// RestoreMessage 从持久化快照恢复历史消息，不重新执行创建校验。
func RestoreMessage(snapshot MessageSnapshot) *Message {
	return &Message{
		id:             snapshot.ID,
		conversationID: snapshot.ConversationID,
		senderID:       snapshot.SenderID,
		receiverID:     snapshot.ReceiverID,
		messageType:    snapshot.Type,
		text:           snapshot.Text,
		emoji:          snapshot.Emoji,
		media:          snapshot.Media.clone(),
		createdAt:      snapshot.CreatedAt,
	}
}

// Snapshot 返回消息快照的副本。修改返回值不会影响消息本身。
func (msg *Message) Snapshot() MessageSnapshot {
	if msg == nil {
		return MessageSnapshot{}
	}

	return MessageSnapshot{
		ID:             msg.id,
		ConversationID: msg.conversationID,
		SenderID:       msg.senderID,
		ReceiverID:     msg.receiverID,
		Type:           msg.messageType,
		Text:           msg.text,
		Emoji:          msg.emoji,
		Media:          msg.media.clone(),
		CreatedAt:      msg.createdAt,
	}
}

// ID 返回消息 ID。
func (msg *Message) ID() string {
	if msg == nil {
		return ""
	}

	return msg.id
}

// ConversationID 返回确定性会话 ID。
func (msg *Message) ConversationID() string {
	if msg == nil {
		return ""
	}

	return msg.conversationID
}

// SenderID 返回发送方用户 ID。
func (msg *Message) SenderID() uint {
	if msg == nil {
		return 0
	}

	return msg.senderID
}

// ReceiverID 返回接收方用户 ID。
func (msg *Message) ReceiverID() uint {
	if msg == nil {
		return 0
	}

	return msg.receiverID
}

// Type 返回消息类型。
func (msg *Message) Type() string {
	if msg == nil {
		return ""
	}

	return msg.messageType
}

// Text 返回文本内容。
func (msg *Message) Text() string {
	if msg == nil {
		return ""
	}

	return msg.text
}

// Emoji 返回 emoji 内容。
func (msg *Message) Emoji() string {
	if msg == nil {
		return ""
	}

	return msg.emoji
}

// Media 返回媒体载荷的副本。
func (msg *Message) Media() *Media {
	if msg == nil {
		return nil
	}

	return msg.media.clone()
}

// CreatedAt 返回创建时间。
func (msg *Message) CreatedAt() time.Time {
	if msg == nil {
		return time.Time{}
	}

	return msg.createdAt
}

// Validate 保持参与者先于内容的业务校验顺序。
func (msg *Message) Validate() error {
	if err := msg.EnsureParticipants(); err != nil {
		return err
	}

	return msg.ValidateContent()
}

// EnsureParticipants 校验发消息双方。
func (msg *Message) EnsureParticipants() error {
	if msg == nil || msg.senderID == 0 || msg.receiverID == 0 {
		return ErrNotLoggedIn
	}

	if msg.senderID == msg.receiverID {
		return ErrSelfMessage
	}

	return nil
}

// ValidateContent 按消息类型校验内容。
func (msg *Message) ValidateContent() error {
	if msg == nil {
		return ErrUnsupportedMessageType
	}

	switch msg.messageType {
	case MessageTypeText:
		if msg.text == "" {
			return ErrEmptyText
		}

		if utf8.RuneCountInString(msg.text) > maxTextRunes {
			return ErrTextTooLong
		}
	case MessageTypeEmoji:
		if msg.emoji == "" && msg.text == "" {
			return ErrEmptyEmoji
		}
	case MessageTypeImage, MessageTypeSticker:
		if msg.media == nil || msg.media.URL == "" {
			return ErrEmptyMediaURL
		}
	default:
		return ErrUnsupportedMessageType
	}

	return nil
}

// Preview 生成会话最后一条消息的预览文案。
func (msg *Message) Preview() string {
	if msg == nil {
		return ""
	}

	switch msg.messageType {
	case MessageTypeText:
		if msg.text == "" {
			return ""
		}

		if utf8.RuneCountInString(msg.text) <= maxPreviewRunes {
			return msg.text
		}

		runes := []rune(msg.text)

		return string(runes[:maxPreviewRunes]) + "..."
	case MessageTypeImage:
		return "[图片]"
	case MessageTypeEmoji:
		if msg.emoji != "" {
			return msg.emoji
		}

		return "[表情]"
	case MessageTypeSticker:
		return "[表情包]"
	default:
		return "[消息]"
	}
}
