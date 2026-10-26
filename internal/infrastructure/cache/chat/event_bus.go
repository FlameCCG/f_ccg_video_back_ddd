// Package chat 适配私信跨实例事件总线。
//
// event_bus.go 使用 Redis Pub/Sub 将用户事件传到持有 WebSocket 连接的实例。
// Pub/Sub 不保存离线事件；消息历史和未读事实仍由持久化查询提供。
//
// 本包实现所属上下文声明的端口，序列化和 Redis 命令留在外层；改变 key 需考虑已存在的数据。
package chat

import (
	"context"
	"f_ccg_video_back_ddd/pkg/constants"
	"f_ccg_video_back_ddd/pkg/logctx"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// publishTimeout 单次事件发布超时（与原 ws_hub.publishToUser 一致）。
const publishTimeout = 2 * time.Second

// EventBus 基于 Redis Pub/Sub 的私信事件总线（原 ws_hub 中直接使用 global.RedisClient 的部分）。
//
// Redis 频道格式 chat:user:{userId} 与原实现完全一致；client 为 nil 时整体降级为单实例模式
// （只在本进程内派发，不跨实例广播）。
type EventBus struct {
	redis  *redis.Client
	logger *zap.Logger
}

// NewEventBus 构造私信事件总线；redisClient 允许为 nil。
func NewEventBus(redisClient *redis.Client, logger *zap.Logger) *EventBus {
	return &EventBus{redis: redisClient, logger: logger}
}

// Available 报告 Redis 是否可用。
func (b *EventBus) Available() bool {
	return b != nil && b.redis != nil
}

// userChannel 返回用户事件频道名。
func userChannel(userID uint) string {
	return constants.ChatUserChannelPrefix + strconv.FormatUint(uint64(userID), 10)
}

// Publish 向目标用户频道发布事件；失败仅告警（最佳努力广播）。
func (b *EventBus) Publish(ctx context.Context, userID uint, payload []byte) {
	if !b.Available() || userID == 0 {
		return
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	pubCtx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()

	if err := b.redis.Publish(pubCtx, userChannel(userID), payload).Err(); err != nil {
		logctx.With(ctx, b.logger).Warn("publish chat event failed", zap.String("event", "cache.event_bus.publish.degraded"), zap.String("operation", "cache.event_bus.publish"), zap.Error(err), zap.Uint("user_id", userID))
	}
}

// Subscribe 订阅用户事件频道，收到消息时回调 handler；返回停止订阅的函数。
//
// 订阅器是房间级后台循环，不绑定单个 HTTP 请求，因此使用独立的 Background 上下文；
// 停止时先 cancel 再显式 Close PubSub，避免仅 cancel 时 Channel 挂住导致泄漏。
// Redis 不可用时返回 nil，调用方据此保持单实例模式。
func (b *EventBus) Subscribe(userID uint, handler func(payload []byte)) func() {
	if !b.Available() || handler == nil {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	pubsub := b.redis.Subscribe(ctx, userChannel(userID))

	go func() {
		defer func() { _ = pubsub.Close() }()

		for msg := range pubsub.Channel() {
			handler([]byte(msg.Payload))
		}
	}()

	var once sync.Once

	return func() {
		once.Do(func() {
			cancel()

			if err := pubsub.Close(); err != nil {
				b.logger.Warn("close chat redis subscriber failed", zap.String("event", "cache.event_bus.subscribe.degraded"), zap.String("operation", "cache.event_bus.subscribe"),
					zap.Error(err),
					zap.Uint("user_id", userID),
				)
			}
		})
	}
}
