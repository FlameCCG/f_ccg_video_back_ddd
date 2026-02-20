package mongopo

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ChatMessageMedia 类型.
type ChatMessageMedia struct {
	URL    string `bson:"url" json:"url"`
	Mime   string `bson:"mime,omitempty" json:"mime,omitempty"`
	Width  int    `bson:"width,omitempty" json:"width,omitempty"`
	Height int    `bson:"height,omitempty" json:"height,omitempty"`
	Size   int64  `bson:"size,omitempty" json:"size,omitempty"`
}

// ChatMessage 私信消息.
type ChatMessage struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ConversationID string             `bson:"conversationId" json:"conversationId"`
	SenderID       uint               `bson:"senderId" json:"senderId"`
	ReceiverID     uint               `bson:"receiverId" json:"receiverId"`
	Type           string             `bson:"type" json:"type"` // text/image/emoji/sticker
	Text           string             `bson:"text,omitempty" json:"text,omitempty"`
	Emoji          string             `bson:"emoji,omitempty" json:"emoji,omitempty"`
	Media          *ChatMessageMedia  `bson:"media,omitempty" json:"media,omitempty"`
	CreatedAt      time.Time          `bson:"createdAt" json:"createdAt"`
}
