// Package mongopo 定义私信会话与消息的 MongoDB BSON 持久化形状。
//
// 会话以两位参与者生成的固定键作为 _id，消息使用 ObjectID；两种标识的排序和唯一性语义不同。
// 成员的可见性、未读数和已读游标与最后一条消息摘要存储在会话文档，消息正文另存。
// bson 标签控制落库键，json 标签不能替代应用读模型的接口设计；转换与索引在 persistence/chat。
// 这些结构体依赖 Mongo 类型，领域中的会话和消息不应直接复用它们。
package mongopo

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ChatConversationMember 保存某个参与者的可见性、未读数与已读游标，同一会话的两位成员可有不同状态。
type ChatConversationMember struct {
	UserID        uint                `bson:"userId" json:"userId"`
	Unread        int64               `bson:"unread" json:"unread"`
	Visible       bool                `bson:"visible,omitempty" json:"-"`
	LastReadAt    *time.Time          `bson:"lastReadAt,omitempty" json:"lastReadAt,omitempty"`
	LastReadMsgID *primitive.ObjectID `bson:"lastReadMsgId,omitempty" json:"lastReadMsgId,omitempty"`
}

// ChatConversationLastMessage 是会话列表使用的最后消息摘要，完整正文仍保存在消息集合。
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
