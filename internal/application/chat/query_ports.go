package chat

import "context"

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
// 实现在本上下文的 infrastructure/persistence/chat，可读取 user_focus 表；关注关系的写侧归 social 上下文。
type FollowRelationQuery interface {
	// IsFollowing 判断 userID 是否关注了 targetUserID；存储不可用时返回 (false, nil)。
	IsFollowing(ctx context.Context, userID, targetUserID uint) (bool, error)
}
