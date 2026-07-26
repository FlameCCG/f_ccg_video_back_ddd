// Package chat 是私信上下文的持久化实现：Mongo 会话/消息仓储、读侧查询、
// 关注关系读侧投影与启动期索引初始化组件。
package chat

// Mongo 集合名（与原 chat_service 完全一致，属于存储契约）。
const (
	chatMessagesCol      = "chat_messages"
	chatConversationsCol = "chat_conversations"
)

// Mongo/BSON 字段与操作符 key（原 service/chat_service/fields.go）。
const (
	fieldConversationID = "conversationId"
	fieldID             = "_id"
	fieldCreatedAt      = "createdAt"
	fieldUpdatedAt      = "updatedAt"
	fieldParticipants   = "participants"
	fieldLastMessage    = "lastMessage"
	fieldSenderID       = "senderId"
	fieldReceiverID     = "receiverId"
	fieldMemberAUserID  = "memberA.userId"
	fieldMemberBUserID  = "memberB.userId"
	fieldMemberAUnread  = "memberA.unread"
	fieldMemberBUnread  = "memberB.unread"
	opSet               = "$set"
	opSetOnInsert       = "$setOnInsert"
	opInc               = "$inc"
	exprMemberAUserID   = "$memberA.userId"
	exprMemberAUnread   = "$memberA.unread"
	exprMemberBUnread   = "$memberB.unread"
	exprMemberBUserID   = "$memberB.userId"
)

// 索引名（Mongo 查询 hint 依赖这些名字，属于存储契约）。
const (
	indexConversationIDDesc          = "conversationId_id_desc"
	indexConversationSenderReceiver  = "conversationId_sender_receiver_createdAt_desc"
	indexReceiverIDDesc              = "receiverId_id_desc"
	indexParticipantsUpdatedAtDesc   = "participants_updatedAt_desc"
	suffixVisible                    = ".visible"
	suffixUnread                     = ".unread"
	suffixUserID                     = ".userId"
	suffixLastReadAt                 = ".lastReadAt"
)
