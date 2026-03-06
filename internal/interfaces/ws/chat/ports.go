// Package chat 管理私信 WebSocket 连接和实时事件分发。
//
// Hub 负责升级、连接登记、读写循环与关闭，ports.go 声明事件总线等外部能力。
// 连接按用户归属，事件总线把其他实例上的消息通知送到本实例；历史记录通过应用查询取得。
// 长连接读写有独立生命周期，HTTP 请求返回不代表连接退出；Hub 需在 Redis 和底层依赖关闭前停止。
// 实时广播与持久化事实分开，连接断开或 Pub/Sub 丢失事件后需重新查询，不能将广播当作持久队列。
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
