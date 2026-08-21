package video

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// danmuPublishTimeout 单次事件发布超时（与原 publishDanmuEnvelope 一致）。
	danmuPublishTimeout = 2 * time.Second
	// danmuOnlineKeyTTL 房间在线集合 TTL（与原 onlineKeyTTL 一致）。
	danmuOnlineKeyTTL = time.Hour
)

// DanmuEventBus 弹幕事件跨实例广播与在线人数统计的 Redis 适配器
// （原 service/video_service/danmu_hub.go 中直接使用 global.RedisClient 的部分）。
//
// Redis 频道 danmu:channel:{videoId}:{partId} 与在线集合 danmu:online:{videoId}:{partId}
// 与原实现完全一致；client 为 nil 时降级为单实例模式（仅本进程派发 + 本地在线数）。
type DanmuEventBus struct {
	redis  *redis.Client
	logger *zap.Logger
}

// NewDanmuEventBus 构造弹幕事件总线；redisClient 允许为 nil。
func NewDanmuEventBus(redisClient *redis.Client, logger *zap.Logger) *DanmuEventBus {
	return &DanmuEventBus{redis: redisClient, logger: logger}
}

// Available 报告 Redis 是否可用。
func (b *DanmuEventBus) Available() bool {
	return b != nil && b.redis != nil
}

// Publish 向房间频道发布事件负载；失败仅告警（最佳努力广播）。
func (b *DanmuEventBus) Publish(ctx context.Context, videoID, partID uint, payload []byte) {
	if !b.Available() {
		return
	}

	pubCtx, cancel := context.WithTimeout(orTODO(ctx), danmuPublishTimeout)
	defer cancel()

	if err := b.redis.Publish(pubCtx, danmuChannelKey(videoID, partID), payload).Err(); err != nil {
		b.logger.Warn("publish danmu event failed", zap.Error(err), zap.Uint("videoId", videoID))
	}
}

// Subscribe 订阅房间频道并在收到消息时回调 handler；返回停止订阅的函数。
//
// 订阅器是房间级后台循环，不绑定单个 HTTP 请求，因此使用独立的 Background 上下文；
// 停止时先 cancel 再显式 Close PubSub，避免仅 cancel 时 Channel 挂住导致泄漏。
// Redis 不可用时返回 nil，调用方据此保持单实例模式。
func (b *DanmuEventBus) Subscribe(videoID, partID uint, handler func(payload []byte)) func() {
	if !b.Available() || handler == nil {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	pubsub := b.redis.Subscribe(ctx, danmuChannelKey(videoID, partID))

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
				b.logger.Warn("close danmu redis subscriber failed",
					zap.Error(err),
					zap.Uint("videoId", videoID),
					zap.Uint("partId", partID),
				)
			}
		})
	}
}

// AddOnline 把用户加入房间在线集合并返回集合基数；失败返回 (0, false)。
func (b *DanmuEventBus) AddOnline(ctx context.Context, videoID, partID, userID uint) (int64, bool) {
	if !b.Available() || userID == 0 {
		return 0, false
	}

	opCtx, cancel := context.WithTimeout(orTODO(ctx), danmuPublishTimeout)
	defer cancel()

	key := danmuOnlineKey(videoID, partID)

	pipe := b.redis.TxPipeline()
	pipe.SAdd(opCtx, key, onlineMember(userID))
	pipe.Expire(opCtx, key, danmuOnlineKeyTTL)

	cardCmd := pipe.SCard(opCtx, key)
	if _, err := pipe.Exec(opCtx); err != nil {
		b.logger.Warn("update danmu online failed", zap.Error(err), zap.Uint("videoId", videoID))

		return 0, false
	}

	return cardCmd.Val(), true
}

// RemoveOnline 把用户移出房间在线集合并返回集合基数；失败返回 (0, false)。
func (b *DanmuEventBus) RemoveOnline(ctx context.Context, videoID, partID, userID uint) (int64, bool) {
	if !b.Available() || userID == 0 {
		return 0, false
	}

	opCtx, cancel := context.WithTimeout(orTODO(ctx), danmuPublishTimeout)
	defer cancel()

	key := danmuOnlineKey(videoID, partID)

	pipe := b.redis.TxPipeline()
	pipe.SRem(opCtx, key, onlineMember(userID))
	pipe.Expire(opCtx, key, danmuOnlineKeyTTL)

	cardCmd := pipe.SCard(opCtx, key)
	if _, err := pipe.Exec(opCtx); err != nil {
		b.logger.Warn("drop danmu online failed", zap.Error(err), zap.Uint("videoId", videoID))

		return 0, false
	}

	return cardCmd.Val(), true
}

// CountOnline 读取房间在线集合基数；失败返回 (0, false)。
func (b *DanmuEventBus) CountOnline(ctx context.Context, videoID, partID uint) (int64, bool) {
	if !b.Available() {
		return 0, false
	}

	opCtx, cancel := context.WithTimeout(orTODO(ctx), danmuPublishTimeout)
	defer cancel()

	val, err := b.redis.SCard(opCtx, danmuOnlineKey(videoID, partID)).Result()
	if err != nil {
		b.logger.Warn("count danmu online failed", zap.Error(err), zap.Uint("videoId", videoID))

		return 0, false
	}

	return val, true
}

// danmuChannelKey 房间事件频道名。
func danmuChannelKey(videoID, partID uint) string {
	return fmt.Sprintf("%s%d:%d", constants.DanmuChannelPrefix, videoID, partID)
}

// danmuOnlineKey 房间在线集合 key。
func danmuOnlineKey(videoID, partID uint) string {
	return fmt.Sprintf("%s%d:%d", constants.DanmuOnlineZSetPrefix, videoID, partID)
}

// onlineMember 在线集合成员名（以 userID 去重统计「在线用户」）。
func onlineMember(userID uint) string {
	return strconv.FormatUint(uint64(userID), 10)
}
