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
	"strconv"
	"time"

	app "f_ccg_video_back_ddd/internal/application/contentops"
	contentopsdomain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// 可见轮播图缓存的 TTL 与超时属于缓存协议。
const (
	visibleBannerCacheTTL     = 10 * time.Minute
	visibleBannerRedisTimeout = 300 * time.Millisecond
	visibleBannerScanCount    = 100
)

// BannerCache 可见轮播图缓存的 Redis 实现。
//
// Redis 客户端在构造时注入。缓存使用 banner:visible: 前缀、10 分钟 TTL 和 300ms 独立超时；
// 缓存 JSON 结构属于数据契约。
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
func (c *BannerCache) LoadVisible(bannerType enum.BannerType, partitionID *uint) ([]contentopsdomain.Banner, bool) {
	if c.redis == nil {
		return nil, false
	}

	key := visibleBannerCacheKey(bannerType, partitionID)

	cacheCtx, cancel := context.WithTimeout(context.Background(), visibleBannerRedisTimeout)
	defer cancel()

	raw, err := c.redis.Get(cacheCtx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.logger.Warn("get visible banner cache failed", zap.String("key", key), zap.Error(err))
		}

		return nil, false
	}

	var items []app.BannerDTO
	if err := json.Unmarshal(raw, &items); err != nil {
		c.logger.Warn("unmarshal visible banner cache failed", zap.String("key", key), zap.Error(err))
		_ = c.redis.Del(cacheCtx, key).Err()

		return nil, false
	}

	return c.bannersFromDTO(items), true
}

// StoreVisible 回填可见轮播图缓存（失败仅告警）。
func (c *BannerCache) StoreVisible(bannerType enum.BannerType, partitionID *uint, items []contentopsdomain.Banner) {
	if c.redis == nil {
		return
	}

	key := visibleBannerCacheKey(bannerType, partitionID)

	raw, err := json.Marshal(c.dtosFromBanners(items))
	if err != nil {
		c.logger.Warn("marshal visible banner cache failed", zap.String("key", key), zap.Error(err))

		return
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), visibleBannerRedisTimeout)
	defer cancel()

	if err := c.redis.Set(cacheCtx, key, raw, visibleBannerCacheTTL).Err(); err != nil {
		c.logger.Warn("set visible banner cache failed", zap.String("key", key), zap.Error(err))
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
		c.logger.Warn("invalidate visible banner cache failed", zap.Error(err))
	}
}

// dtosFromBanners 按缓存 JSON 结构编码，nil 与空切片分别保持。
func (*BannerCache) dtosFromBanners(items []contentopsdomain.Banner) []app.BannerDTO {
	return (app.BannerDTO{}).FromBanners(items)
}

// bannersFromDTO 把已有缓存解码为领域实体。不重新执行创建校验。
func (*BannerCache) bannersFromDTO(items []app.BannerDTO) []contentopsdomain.Banner {
	// 缓存允许写入 null，但读取时分配空切片，因此 HTTP 输出 []。
	out := make([]contentopsdomain.Banner, len(items))
	for i := range items {
		out[i] = *contentopsdomain.RestoreBanner(contentopsdomain.BannerSnapshot{
			ID:          items[i].ID,
			Cover:       items[i].Cover,
			Href:        items[i].Href,
			Show:        items[i].Show,
			Type:        items[i].Type,
			PartitionID: items[i].PartitionID,
		})
	}

	return out
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
