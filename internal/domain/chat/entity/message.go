package entity

import (
	"time"
	"unicode/utf8"

	chaterrors "f_ccg_video_back_ddd/internal/domain/chat/domainerr"
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

// Media 消息附带的媒体信息。
type Media struct {
	URL    string
	Mime   string
	Width  int
	Height int
	Size   int64
}

// Message 私信消息实体。
//
// ID 为 Mongo ObjectID 的十六进制表示，由仓储在落库时生成并回填，
// 领域层不依赖具体存储的 ID 类型。
type Message struct {
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

// NewMessage 在构造边界统一检查参与者和内容，并生成确定性的会话身份。
func NewMessage(senderID, receiverID uint, msgType, text, emoji string, media *Media, now time.Time) (*Message, error) {
	msg := &Message{
		SenderID: senderID, ReceiverID: receiverID, Type: msgType,
		Text: text, Emoji: emoji, Media: media, CreatedAt: now,
	}
	if err := msg.Validate(); err != nil {
		return nil, err
	}
	msg.ConversationID = BuildConversationID(senderID, receiverID)
	if media != nil {
		copy := *media
		msg.Media = &copy
	}
	return msg, nil
}

// Validate 保持参与者先于内容的业务校验顺序。
func (msg *Message) Validate() error {
	if err := msg.EnsureParticipants(); err != nil {
		return err
	}
	return msg.ValidateContent()
}

// ValidateContent 按消息类型校验内容。
func (msg *Message) ValidateContent() error {
	msgType, text, emoji, media := msg.Type, msg.Text, msg.Emoji, msg.Media
	switch msgType {
	case MessageTypeText:
		if text == "" {
			return chaterrors.ErrEmptyText
		}

		if utf8.RuneCountInString(text) > maxTextRunes {
			return chaterrors.ErrTextTooLong
		}
	case MessageTypeEmoji:
		if emoji == "" && text == "" {
			return chaterrors.ErrEmptyEmoji
		}
	case MessageTypeImage, MessageTypeSticker:
		if media == nil || media.URL == "" {
			return chaterrors.ErrEmptyMediaURL
		}
	default:
		return chaterrors.ErrUnsupportedMessageType
	}

	return nil
}

// Preview 生成会话最后一条消息的预览文案。
func (msg *Message) Preview() string {
	if msg == nil {
		return ""
	}

	switch msg.Type {
	case MessageTypeText:
		if msg.Text == "" {
			return ""
		}

		if utf8.RuneCountInString(msg.Text) <= maxPreviewRunes {
			return msg.Text
		}

		runes := []rune(msg.Text)

		return string(runes[:maxPreviewRunes]) + "..."
	case MessageTypeImage:
		return "[图片]"
	case MessageTypeEmoji:
		if msg.Emoji != "" {
			return msg.Emoji
		}

		return "[表情]"
	case MessageTypeSticker:
		return "[表情包]"
	default:
		return "[消息]"
	}
}
