// Package chat 是私信上下文的缓存/消息适配器：跨实例 WebSocket 事件广播。
package chat

import (
	"context"
	"strconv"
	"sync"
	"time"

	"f_ccg_video_back_ddd/pkg/constants"

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
		b.logger.Warn("publish chat event failed", zap.Error(err), zap.Uint("userId", userID))
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
				b.logger.Warn("close chat redis subscriber failed",
					zap.Error(err),
					zap.Uint("userId", userID),
				)
			}
		})
	}
}
