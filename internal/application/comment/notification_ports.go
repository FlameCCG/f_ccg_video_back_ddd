package comment

import (
	"context"
)

// Notifier 站内通知写入端口（跨上下文：notification 写侧）。
//
// 由 notification 上下文的应用服务在 bootstrap 中绑定实现。
// 一次批量 INSERT，随后按去重后的接收者逐个失效角标缓存。
type Notifier interface {
	// CreateNotices 批量写入通知（一次插入，失败则整批不生效）。
	CreateNotices(ctx context.Context, notices []Notice) error
	// InvalidateCount 失效指定接收者的角标计数缓存。
	InvalidateCount(receiverID int)
}
