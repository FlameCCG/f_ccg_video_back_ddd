// Package notification 适配用户通知角标的派生计数缓存。
//
// count_cache.go 保存计数并提供失效能力，实际通知和私信未读事实由各自仓储查询。
// 新增通知、删除和已读操作需要配合失效；不要把旧角标当成授权或消息存在性的依据。
//
// 本包实现所属上下文声明的端口，序列化和 Redis 命令留在外层；改变 key 需考虑已存在的数据。
package notification

import (
	"context"
	"encoding/json"
	"errors"
	"f_ccg_video_back_ddd/pkg/constants"
	"strconv"
	"time"

	app "f_ccg_video_back_ddd/internal/application/notification"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// 角标 count 是热路径：短 fresh + 稍长 stale，写后主动失效。
	notificationCountCacheTTL      = 5 * time.Second
	notificationCountCacheStaleTTL = time.Minute
	notificationCountRedisTimeout  = 300 * time.Millisecond
	// scanBatchSize 全量清理时的 SCAN 批量大小（与原实现一致）。
	scanBatchSize = 100
)

// redisValue redis 载荷：用 cachedAt 区分 fresh / stale-while-revalidate。
type redisValue struct {
	Data     app.CountSummary `json:"data"`
	CachedAt int64            `json:"cachedAt"` // unix nano
}

// CountCache 角标计数缓存的 Redis 实现（原 service/notification_service/cache.go）。
//
// Redis key 前缀与 TTL 与原实现完全一致。
type CountCache struct {
	redis  *redis.Client
	logger *zap.Logger
}

var _ app.CountCache = (*CountCache)(nil)

// NewCountCache 构造角标计数缓存适配器。
func NewCountCache(redisClient *redis.Client, logger *zap.Logger) *CountCache {
	return &CountCache{redis: redisClient, logger: logger}
}

// Available 报告 Redis 是否可用。
func (c *CountCache) Available() bool {
	return c.redis != nil
}

func cacheKey(receiverID int) string {
	return constants.NotificationCountCacheKeyPrefix + strconv.Itoa(receiverID)
}

// Load 读取缓存；返回值分别为数据、是否新鲜、是否命中。
func (c *CountCache) Load(receiverID int) (*app.CountSummary, bool, bool) {
	if c.redis == nil {
		return nil, false, false
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), notificationCountRedisTimeout)
	defer cancel()

	key := cacheKey(receiverID)

	raw, err := c.redis.Get(cacheCtx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.logger.Warn("get notification count cache failed", zap.String("event", "cache.count_cache.load.degraded"), zap.String("operation", "cache.count_cache.load"), zap.Int("receiver_id", receiverID), zap.Error(err))
		}

		return nil, false, false
	}

	var payload redisValue
	if err := json.Unmarshal(raw, &payload); err != nil {
		c.logger.Warn("unmarshal notification count cache failed", zap.String("event", "cache.count_cache.load.degraded"), zap.String("operation", "cache.count_cache.load"), zap.Int("receiver_id", receiverID), zap.Error(err))
		_ = c.redis.Del(cacheCtx, key).Err()

		return nil, false, false
	}

	age := time.Since(time.Unix(0, payload.CachedAt))
	if age > notificationCountCacheStaleTTL || payload.CachedAt == 0 {
		_ = c.redis.Del(cacheCtx, key).Err()

		return nil, false, false
	}

	data := payload.Data
	fresh := age <= notificationCountCacheTTL

	return &data, fresh, true
}

// Store 写入缓存（stale TTL 作为 Redis 过期时间）。
func (c *CountCache) Store(receiverID int, data app.CountSummary) {
	if c.redis == nil {
		return
	}

	payload := redisValue{
		Data:     data,
		CachedAt: time.Now().UnixNano(),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		c.logger.Warn("marshal notification count cache failed", zap.String("event", "cache.count_cache.store.degraded"), zap.String("operation", "cache.count_cache.store"), zap.Int("receiver_id", receiverID), zap.Error(err))

		return
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), notificationCountRedisTimeout)
	defer cancel()

	if err := c.redis.Set(cacheCtx, cacheKey(receiverID), raw, notificationCountCacheStaleTTL).Err(); err != nil {
		c.logger.Warn("set notification count cache failed", zap.String("event", "cache.count_cache.store.degraded"), zap.String("operation", "cache.count_cache.store"), zap.Int("receiver_id", receiverID), zap.Error(err))
	}
}

// Invalidate 删除指定接收者的计数缓存。
func (c *CountCache) Invalidate(receiverID int) {
	if c.redis == nil {
		return
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), notificationCountRedisTimeout)
	defer cancel()

	if err := c.redis.Del(cacheCtx, cacheKey(receiverID)).Err(); err != nil && !errors.Is(err, redis.Nil) {
		c.logger.Warn("invalidate notification count cache failed", zap.String("event", "cache.count_cache.invalidate.degraded"), zap.String("operation", "cache.count_cache.invalidate"), zap.Int("receiver_id", receiverID), zap.Error(err))
	}
}

// InvalidateAll 清除全部计数缓存（全员广播场景）。
func (c *CountCache) InvalidateAll() {
	if c.redis == nil {
		return
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), notificationCountRedisTimeout)
	defer cancel()

	if err := c.deleteKeysByPrefix(cacheCtx, constants.NotificationCountCacheKeyPrefix); err != nil {
		c.logger.Warn("invalidate all notification count cache failed", zap.String("event", "cache.count_cache.invalidate_all.degraded"), zap.String("operation", "cache.count_cache.invalidate_all"), zap.Error(err))
	}
}

func (c *CountCache) deleteKeysByPrefix(ctx context.Context, prefix string) error {
	if c.redis == nil || prefix == "" {
		return nil
	}

	var cursor uint64

	pattern := prefix + "*"

	for {
		keys, next, err := c.redis.Scan(ctx, cursor, pattern, scanBatchSize).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			if err := c.redis.Del(ctx, keys...).Err(); err != nil && !errors.Is(err, redis.Nil) {
				return err
			}
		}

		cursor = next
		if cursor == 0 {
			break
		}
	}

	return nil
}
