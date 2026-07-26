// Package mongopo provides package functionality.
package mongopo

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ChatConversationMember 类型.
type ChatConversationMember struct {
	UserID        uint                `bson:"userId" json:"userId"`
	Unread        int64               `bson:"unread" json:"unread"`
	Visible       bool                `bson:"visible,omitempty" json:"-"`
	LastReadAt    *time.Time          `bson:"lastReadAt,omitempty" json:"lastReadAt,omitempty"`
	LastReadMsgID *primitive.ObjectID `bson:"lastReadMsgId,omitempty" json:"lastReadMsgId,omitempty"`
}

// ChatConversationLastMessage 类型.
type ChatConversationLastMessage struct {
	MsgID     primitive.ObjectID `bson:"msgId" json:"msgId"`
	Type      string             `bson:"type" json:"type"`
	Preview   string             `bson:"preview" json:"preview"`
	SenderID  uint               `bson:"senderId" json:"senderId"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

// ChatConversation 私信会话（两人固定会话；_id 为 deterministic conversationKey）.
type ChatConversation struct {
	ID           string                       `bson:"_id" json:"id"`
	Participants []uint                       `bson:"participants" json:"participants"`
	MemberA      ChatConversationMember       `bson:"memberA" json:"memberA"`
	MemberB      ChatConversationMember       `bson:"memberB" json:"memberB"`
	LastMessage  *ChatConversationLastMessage `bson:"lastMessage,omitempty" json:"lastMessage,omitempty"`
	CreatedAt    time.Time                    `bson:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time                    `bson:"updatedAt" json:"updatedAt"`
}
