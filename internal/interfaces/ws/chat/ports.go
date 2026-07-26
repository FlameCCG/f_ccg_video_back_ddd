// Package chat 是私信 WebSocket 端点：连接升级、房间管理与跨实例事件广播。
package chat

import "context"

// EventBus 私信事件跨实例广播端口。
//
// 由 infrastructure/cache/chat.EventBus（Redis Pub/Sub）在 bootstrap 中绑定实现；
// 端口声明在消费方（接口层），使 hub 不直接依赖 Redis 客户端。
type EventBus interface {
	// Available 报告底层广播通道是否可用；不可用时 hub 退化为单实例派发。
	Available() bool
	// Publish 向目标用户频道发布事件负载（最佳努力，失败仅记录日志）。
	Publish(ctx context.Context, userID uint, payload []byte)
	// Subscribe 订阅用户频道并在收到消息时回调 handler；返回停止订阅的函数。
	// 通道不可用时返回 nil。
	Subscribe(userID uint, handler func(payload []byte)) func()
}
