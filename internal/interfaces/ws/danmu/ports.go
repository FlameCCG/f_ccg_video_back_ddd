// Package danmu 管理弹幕 WebSocket 连接和实时事件分发。
//
// Hub 负责升级、连接登记、读写循环与关闭，ports.go 声明事件总线等外部能力。
// 连接按视频/分P房间组织，并维护在线信息、最近弹幕与跨实例事件转发。
// 长连接读写有独立生命周期，HTTP 请求返回不代表连接退出；Hub 需在 Redis 和底层依赖关闭前停止。
// 实时广播与持久化事实分开，连接断开或 Pub/Sub 丢失事件后需重新查询，不能将广播当作持久队列。
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
