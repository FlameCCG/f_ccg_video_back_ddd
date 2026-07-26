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

// ValidateContent 按消息类型校验内容（分支与文案顺序与原 SendMessage 完全一致）。
func ValidateContent(msgType, text, emoji string, media *Media) error {
	switch msgType {
	case MessageTypeText:
		if text == "" {
			return ErrEmptyText
		}

		if utf8.RuneCountInString(text) > maxTextRunes {
			return ErrTextTooLong
		}
	case MessageTypeEmoji:
		if emoji == "" && text == "" {
			return ErrEmptyEmoji
		}
	case MessageTypeImage, MessageTypeSticker:
		if media == nil || media.URL == "" {
			return ErrEmptyMediaURL
		}
	default:
		return ErrUnsupportedMessageType
	}

	return nil
}

// BuildPreview 生成会话最后一条消息的预览文案。
func BuildPreview(msg *Message) string {
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
