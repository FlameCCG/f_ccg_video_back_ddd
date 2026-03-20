package notification

import (
	"context"
)

// CountCache 角标计数缓存端口（Redis，fresh / stale-while-revalidate）。
type CountCache interface {
	// Available 报告底层缓存是否可用。
	Available() bool
	// Load 读取缓存；返回值分别为数据、是否新鲜、是否命中。
	Load(receiverID int) (*NotificationCountSummary, bool, bool)
	// Store 写入缓存。
	Store(receiverID int, data NotificationCountSummary)
	// Invalidate 删除指定接收者的计数缓存。
	Invalidate(receiverID int)
	// InvalidateAll 清除全部计数缓存（全员广播场景）。
	InvalidateAll()
}

// ChatUnreadCounter 聊天未读数查询端口。
//
// 由 chat 上下文的应用服务在 bootstrap 中绑定实现，实现跨上下文协调。
type ChatUnreadCounter interface {
	TotalUnread(ctx context.Context, userID uint) (int64, error)
}
