package video

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// submittablePartitionCacheTTL 可投稿分区列表缓存 TTL。
	submittablePartitionCacheTTL = 10 * time.Minute
	// submittablePartitionRedisTimeout 单次 Redis 操作超时。
	submittablePartitionRedisTimeout = 300 * time.Millisecond
)

// PartitionCache 可投稿分区列表的 Redis 适配器（原 service/video_service/partition_cache.go）。
//
// 端口方法不带 context：与原实现一致，用 context.Background() 派生短超时。
type PartitionCache struct {
	redis  *redis.Client
	logger *zap.Logger
}

var _ app.PartitionCache = (*PartitionCache)(nil)

// NewPartitionCache 构造分区列表缓存适配器；redisClient 允许为 nil。
func NewPartitionCache(redisClient *redis.Client, logger *zap.Logger) *PartitionCache {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &PartitionCache{redis: redisClient, logger: logger}
}

// Load 读取缓存的可投稿分区列表；未命中或脏数据返回 (nil, false)。
func (c *PartitionCache) Load() ([]app.PartitionItem, bool) {
	if c == nil || c.redis == nil {
		return nil, false
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), submittablePartitionRedisTimeout)
	defer cancel()

	raw, err := c.redis.Get(cacheCtx, constants.PartitionSubmittableCacheKey).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.logger.Warn("get submittable partition cache failed", zap.Error(err))
		}

		return nil, false
	}

	var items []app.PartitionItem
	if err := json.Unmarshal(raw, &items); err != nil {
		c.logger.Warn("unmarshal submittable partition cache failed", zap.Error(err))
		_ = c.redis.Del(cacheCtx, constants.PartitionSubmittableCacheKey).Err()

		return nil, false
	}

	copied := make([]app.PartitionItem, len(items))
	copy(copied, items)

	return copied, true
}

// Store 写入可投稿分区列表缓存。
func (c *PartitionCache) Store(items []app.PartitionItem) {
	if c == nil || c.redis == nil {
		return
	}

	raw, err := json.Marshal(items)
	if err != nil {
		c.logger.Warn("marshal submittable partition cache failed", zap.Error(err))

		return
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), submittablePartitionRedisTimeout)
	defer cancel()

	if err := c.redis.Set(
		cacheCtx,
		constants.PartitionSubmittableCacheKey,
		raw,
		submittablePartitionCacheTTL,
	).Err(); err != nil {
		c.logger.Warn("set submittable partition cache failed", zap.Error(err))
	}
}

// Invalidate 删除可投稿分区列表缓存。
func (c *PartitionCache) Invalidate() {
	if c == nil || c.redis == nil {
		return
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), submittablePartitionRedisTimeout)
	defer cancel()

	if err := c.redis.Del(
		cacheCtx,
		constants.PartitionSubmittableCacheKey,
	).Err(); err != nil && !errors.Is(err, redis.Nil) {
		c.logger.Warn("invalidate submittable partition cache failed", zap.Error(err))
	}
}
