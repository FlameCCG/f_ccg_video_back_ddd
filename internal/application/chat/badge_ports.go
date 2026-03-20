package chat

// NotificationBadgeInvalidator 通知角标缓存失效端口（跨上下文，写侧）。
//
// 由 notification 上下文的应用服务在 bootstrap 中绑定实现
// （*notification.CountService 的 InvalidateCount 方法），避免 chat 反向 import 通知包。
type NotificationBadgeInvalidator interface {
	// InvalidateCount 失效指定接收者的角标计数缓存。
	InvalidateCount(receiverID int)
}
