// Package danmu 是弹幕 WebSocket 端点：连接升级、房间管理、在线人数与跨实例事件广播。
package danmu

import "context"

// EventBus 弹幕事件跨实例广播与在线人数统计端口。
//
// 由 infrastructure/cache/video.DanmuEventBus（Redis Pub/Sub + Set）在 bootstrap 中绑定实现；
// 端口声明在消费方（接口层），使 hub 不直接依赖 Redis 客户端。
type EventBus interface {
	// Available 报告底层广播通道是否可用；不可用时 hub 退化为单实例派发 + 本地在线数。
	Available() bool
	// Publish 向房间频道发布事件负载（最佳努力，失败仅记录日志）。
	Publish(ctx context.Context, videoID, partID uint, payload []byte)
	// Subscribe 订阅房间频道并在收到消息时回调 handler；返回停止订阅的函数。
	// 通道不可用时返回 nil。
	Subscribe(videoID, partID uint, handler func(payload []byte)) func()
	// AddOnline 把用户加入房间在线集合并返回集合基数；失败返回 ok=false。
	AddOnline(ctx context.Context, videoID, partID, userID uint) (int64, bool)
	// RemoveOnline 把用户移出房间在线集合并返回集合基数；失败返回 ok=false。
	RemoveOnline(ctx context.Context, videoID, partID, userID uint) (int64, bool)
	// CountOnline 读取房间在线集合基数；失败返回 ok=false。
	CountOnline(ctx context.Context, videoID, partID uint) (int64, bool)
}
