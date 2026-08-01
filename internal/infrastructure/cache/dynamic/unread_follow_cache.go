// Package dynamic 是动态上下文的缓存适配器：关注未读数缓存。
package dynamic

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	app "f_ccg_video_back_ddd/internal/application/dynamic"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// 未读数是角标热路径：短 fresh + 稍长 stale，写后主动失效。
	unreadFollowCountCacheTTL      = 10 * time.Second
	unreadFollowCountCacheStaleTTL = time.Minute
	unreadFollowCountRedisTimeout  = 300 * time.Millisecond
)

// redisValue redis 载荷：用 cachedAt 区分 fresh / stale-while-revalidate。
type redisValue struct {
	Count    int64 `json:"count"`
	CachedAt int64 `json:"cachedAt"` // unix nano
}

// UnreadFollowCountCache 关注未读数缓存的 Redis 实现（原 service/dynamic_service/cache.go）。
//
// Redis key 前缀与 TTL 与原实现完全一致。
type UnreadFollowCountCache struct {
	redis  *redis.Client
	logger *zap.Logger
}

var _ app.UnreadFollowCountCache = (*UnreadFollowCountCache)(nil)

// NewUnreadFollowCountCache 构造关注未读数缓存适配器。
func NewUnreadFollowCountCache(redisClient *redis.Client, logger *zap.Logger) *UnreadFollowCountCache {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &UnreadFollowCountCache{redis: redisClient, logger: logger}
}

func cacheKey(userID uint) string {
	return constants.DynamicUnreadFollowCacheKeyPrefix + strconv.FormatUint(uint64(userID), 10)
}

// Load 读取缓存；返回值分别为计数、是否新鲜、是否命中。
//
// 载荷损坏或已超出 stale 窗口时顺手删除该键并按未命中处理。
func (c *UnreadFollowCountCache) Load(userID uint) (int64, bool, bool) {
	if c.redis == nil {
		return 0, false, false
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), unreadFollowCountRedisTimeout)
	defer cancel()

	key := cacheKey(userID)

	raw, err := c.redis.Get(cacheCtx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.logger.Warn("get unread follow count cache failed", zap.Uint("userId", userID), zap.Error(err))
		}

		return 0, false, false
	}

	var payload redisValue
	if err := json.Unmarshal(raw, &payload); err != nil {
		c.logger.Warn("unmarshal unread follow count cache failed", zap.Uint("userId", userID), zap.Error(err))
		_ = c.redis.Del(cacheCtx, key).Err()

		return 0, false, false
	}

	age := time.Since(time.Unix(0, payload.CachedAt))
	if age > unreadFollowCountCacheStaleTTL || payload.CachedAt == 0 {
		_ = c.redis.Del(cacheCtx, key).Err()

		return 0, false, false
	}

	fresh := age <= unreadFollowCountCacheTTL

	return payload.Count, fresh, true
}

// Store 写入缓存（stale TTL 作为 Redis 过期时间）。
func (c *UnreadFollowCountCache) Store(userID uint, count int64) {
	if c.redis == nil {
		return
	}

	payload := redisValue{
		Count:    count,
		CachedAt: time.Now().UnixNano(),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		c.logger.Warn("marshal unread follow count cache failed", zap.Uint("userId", userID), zap.Error(err))

		return
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), unreadFollowCountRedisTimeout)
	defer cancel()

	if err := c.redis.Set(cacheCtx, cacheKey(userID), raw, unreadFollowCountCacheStaleTTL).Err(); err != nil {
		c.logger.Warn("set unread follow count cache failed", zap.Uint("userId", userID), zap.Error(err))
	}
}

// Invalidate 删除指定用户的未读数缓存。
func (c *UnreadFollowCountCache) Invalidate(userID uint) {
	if c.redis == nil {
		return
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), unreadFollowCountRedisTimeout)
	defer cancel()

	if err := c.redis.Del(cacheCtx, cacheKey(userID)).Err(); err != nil && !errors.Is(err, redis.Nil) {
		c.logger.Warn("invalidate unread follow count cache failed", zap.Uint("userId", userID), zap.Error(err))
	}
}
