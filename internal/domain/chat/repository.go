package chat

import (
	"context"
	"time"
)

// Repository 私信写侧仓储（接口所有权归领域层，实现在 infrastructure/persistence/chat）。
//
// 存储不可用（Mongo 未配置）时 Available 返回 false，应用服务据此返回
// ErrStoreUnavailable —— 与原实现「每个 service 方法先 getChatDB」的行为顺序一致。
type Repository interface {
	// Available 报告私信存储是否可用。
	Available() bool
	// EnsureIndexes 创建聊天集合索引（启动期幂等执行）。
	EnsureIndexes(ctx context.Context) error
	// InsertMessage 落库一条消息，并回填仓储生成的消息 ID。
	InsertMessage(ctx context.Context, msg *Message) error
	// ApplySend 按消息推进会话：写入 lastMessage、双方可见、接收方未读 +1（upsert）。
	ApplySend(ctx context.Context, msg *Message, preview string) error
	// UpsertConversation 幂等创建/恢复会话，并返回会话当前状态。
	UpsertConversation(ctx context.Context, userID, peerID uint, now time.Time) (*Conversation, error)
	// HideConversation 仅对 userID 隐藏会话并清零其未读，保留对端会话与历史消息。
	HideConversation(ctx context.Context, userID, peerID uint, now time.Time) error
	// MarkRead 清零 userID 在该会话中的未读并记录已读时间；返回是否命中会话。
	MarkRead(ctx context.Context, userID, peerID uint, now time.Time) (bool, error)
	// ExistsMessageBetween 判断指定时间窗内是否存在 senderID → receiverID 的消息。
	ExistsMessageBetween(
		ctx context.Context,
		conversationID string,
		senderID, receiverID uint,
		start, end time.Time,
	) (bool, error)
}
