// Package chat 实现私信 MongoDB 仓储、查询与索引初始化。
//
// store.go/query.go 处理会话与消息，converter.go 隔离 BSON 持久化模型与应用/领域对象。
// 消息插入、会话摘要和未读推进的写入步骤要逐项看错误处理；不能套用 MySQL 的 gormx 事务保证。
// focus_query.go 是关注事实的 SQL 投影；index_initializer.go 将索引准备接入生命周期。
//
// 仓储遵守消费方端口的不存在、错误和 ID 回填约定，不能只看方法名判断 nil 或错误的含义。
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
	indexConversationIDDesc         = "conversationId_id_desc"
	indexConversationSenderReceiver = "conversationId_sender_receiver_createdAt_desc"
	indexReceiverIDDesc             = "receiverId_id_desc"
	indexParticipantsUpdatedAtDesc  = "participants_updatedAt_desc"
	suffixVisible                   = ".visible"
	suffixUnread                    = ".unread"
	suffixLastReadAt                = ".lastReadAt"
)
