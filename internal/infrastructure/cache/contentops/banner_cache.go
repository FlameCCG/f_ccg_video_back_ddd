// Package contentops 适配轮播图缓存与站点访问统计。
//
// banner_cache.go 缓存可见轮播图，site_stat_store.go 记录访问量与访问去重。
// 缓存、计数和持久化表有不同权威来源；错误时是否回源由相应应用用例决定。
//
// 本包实现所属上下文声明的端口，序列化和 Redis 命令留在外层；改变 key 需考虑已存在的数据。
package contentops

import (
	"context"
	"encoding/json"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/constants"
	"strconv"
	"time"

	app "f_ccg_video_back_ddd/internal/application/contentops"
	domain "f_ccg_video_back_ddd/internal/domain/contentops"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	testRedisUninitialized = "redis 未初始化"
)

// 可见轮播图缓存参数（TTL 与超时属外部契约，与原 service/banner_service/cache.go 一致）。
const (
	visibleBannerCacheTTL     = 10 * time.Minute
	visibleBannerRedisTimeout = 300 * time.Millisecond
	visibleBannerScanCount    = 100
)

// BannerCache 可见轮播图缓存的 Redis 实现（原 service/banner_service/cache.go）。
//
// 原实现是一组直读 global.RedisClient 的包级函数；这里改为构造期注入的结构体字段，
// Redis key 前缀（banner:visible:）、10 分钟 TTL、300ms 独立超时与缓存值的
// JSON 结构均与原实现完全一致。
type BannerCache struct {
	redis  *redis.Client
	logger *zap.Logger
}

var _ app.BannerCache = (*BannerCache)(nil)

// NewBannerCache 构造可见轮播图缓存适配器。
func NewBannerCache(redisClient *redis.Client, logger *zap.Logger) *BannerCache {
	return &BannerCache{redis: redisClient, logger: logger}
}

// visibleBannerCacheKey 组装缓存 key：banner:visible:<type>:<partitionId|nil>。
func visibleBannerCacheKey(bannerType enum.BannerType, partitionID *uint) string {
	suffix := strconv.Itoa(int(bannerType)) + ":nil"
	if partitionID != nil {
		suffix = strconv.Itoa(int(bannerType)) + ":" + strconv.FormatUint(uint64(*partitionID), 10)
	}

	return constants.BannerVisibleCacheKeyPrefix + suffix
}

// LoadVisible 读取可见轮播图缓存；未命中、Redis 未初始化或反序列化失败返回 (nil, false)。
func (c *BannerCache) LoadVisible(bannerType enum.BannerType, partitionID *uint) ([]domain.Banner, bool) {
	if c.redis == nil {
		return nil, false
	}

	key := visibleBannerCacheKey(bannerType, partitionID)

	cacheCtx, cancel := context.WithTimeout(context.Background(), visibleBannerRedisTimeout)
	defer cancel()

	raw, err := c.redis.Get(cacheCtx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.logger.Warn("get visible banner cache failed", zap.String("event", "cache.banner_cache.load_visible.degraded"), zap.String("operation", "cache.banner_cache.load_visible"), zap.String("key", key), zap.Error(err))
		}

		return nil, false
	}

	var items []domain.Banner
	if err := json.Unmarshal(raw, &items); err != nil {
		c.logger.Warn("unmarshal visible banner cache failed", zap.String("event", "cache.banner_cache.load_visible.degraded"), zap.String("operation", "cache.banner_cache.load_visible"), zap.String("key", key), zap.Error(err))
		_ = c.redis.Del(cacheCtx, key).Err()

		return nil, false
	}

	// 返回副本，避免调用方修改影响后续逻辑
	copied := make([]domain.Banner, len(items))
	copy(copied, items)

	return copied, true
}

// StoreVisible 回填可见轮播图缓存（失败仅告警）。
func (c *BannerCache) StoreVisible(bannerType enum.BannerType, partitionID *uint, items []domain.Banner) {
	if c.redis == nil {
		return
	}

	key := visibleBannerCacheKey(bannerType, partitionID)

	raw, err := json.Marshal(items)
	if err != nil {
		c.logger.Warn("marshal visible banner cache failed", zap.String("event", "cache.banner_cache.store_visible.degraded"), zap.String("operation", "cache.banner_cache.store_visible"), zap.String("key", key), zap.Error(err))

		return
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), visibleBannerRedisTimeout)
	defer cancel()

	if err := c.redis.Set(cacheCtx, key, raw, visibleBannerCacheTTL).Err(); err != nil {
		c.logger.Warn("set visible banner cache failed", zap.String("event", "cache.banner_cache.store_visible.degraded"), zap.String("operation", "cache.banner_cache.store_visible"), zap.String("key", key), zap.Error(err))
	}
}

// InvalidateVisible 按前缀清空全部可见轮播图缓存（失败仅告警）。
func (c *BannerCache) InvalidateVisible() {
	if c.redis == nil {
		return
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), visibleBannerRedisTimeout)
	defer cancel()

	if err := c.deleteKeysByPrefix(cacheCtx, constants.BannerVisibleCacheKeyPrefix); err != nil {
		c.logger.Warn("invalidate visible banner cache failed", zap.String("event", "cache.banner_cache.invalidate_visible.degraded"), zap.String("operation", "cache.banner_cache.invalidate_visible"), zap.Error(err))
	}
}

// deleteKeysByPrefix 用 SCAN 游标分批删除指定前缀的 key。
func (c *BannerCache) deleteKeysByPrefix(ctx context.Context, prefix string) error {
	if c.redis == nil || prefix == "" {
		return nil
	}

	var cursor uint64

	pattern := prefix + "*"

	for {
		keys, next, err := c.redis.Scan(ctx, cursor, pattern, visibleBannerScanCount).Result()
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
