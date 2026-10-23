package chat

import "context"

// 端口阅读：写侧仓储位于 domain/chat，本文件补充会话列表、关注事实与通知角标等应用协作能力。
// 读模型无需先还原完整写实体，跨通知上下文依赖经 bootstrap 绑定。

// Query 私信读侧查询端口（读模型由 infrastructure/persistence/chat 直接组装）。
type Query interface {
	// ListConversations 会话列表分页（按 updatedAt 倒序），返回读模型与总数。
	ListConversations(ctx context.Context, userID uint, page, pageSize int) ([]ConversationItem, int64, error)
	// TotalUnread 汇总用户在所有可见会话中的未读总数。
	TotalUnread(ctx context.Context, userID uint) (int64, error)
	// ListMessagesBefore 按 before 游标向前拉取消息（降序，最多 limit 条）。
	ListMessagesBefore(ctx context.Context, userID, peerID uint, before string, limit int) ([]MessageItem, error)
}

// FollowRelationQuery 关注关系查询端口（读侧投影）。
//
// 原实现直接在 chat_service 内查 user_focus 表；按 D-011，读侧投影允许直读对方存储，
// 实现放在本上下文的 infrastructure/persistence/chat（关注关系的写侧仍归 social 上下文）。
type FollowRelationQuery interface {
	// IsFollowing 判断 userID 是否关注了 targetUserID；存储不可用时返回 (false, nil)。
	IsFollowing(ctx context.Context, userID, targetUserID uint) (bool, error)
}

// NotificationBadgeInvalidator 通知角标缓存失效端口（跨上下文，写侧）。
//
// 由 notification 上下文的应用服务在 bootstrap 中绑定实现
// （*notification.Service 的 InvalidateCount 方法），避免 chat 反向 import 通知包。
type NotificationBadgeInvalidator interface {
	// InvalidateCount 失效指定接收者的角标计数缓存。
	InvalidateCount(receiverID int)
}
