package video

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// folderListCacheTTL 收藏夹列表缓存 TTL。
	folderListCacheTTL = 15 * time.Second
	// folderListRedisTimeout 单次 Redis 操作超时。
	folderListRedisTimeout = 300 * time.Millisecond
	// folderListScanBatch 前缀清理时的 SCAN 批量。
	folderListScanBatch = 100
)

// FolderListCache 收藏夹列表的 Redis 适配器（原 service/video_service/folder_cache.go）。
//
// 该端口的方法签名不带 context：原实现同样用 context.Background() 派生短超时，
// 保证「即使请求已取消也能完成缓存写入/失效」。
type FolderListCache struct {
	redis  *redis.Client
	logger *zap.Logger
}

var _ app.FolderListCache = (*FolderListCache)(nil)

// NewFolderListCache 构造收藏夹列表缓存适配器；redisClient 允许为 nil。
func NewFolderListCache(redisClient *redis.Client, logger *zap.Logger) *FolderListCache {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &FolderListCache{redis: redisClient, logger: logger}
}

// Load 读取缓存的收藏夹列表；未命中或脏数据返回 (nil, false)。
func (c *FolderListCache) Load(userID, videoID uint) ([]app.FolderWithCount, bool) {
	if c == nil || c.redis == nil {
		return nil, false
	}

	key := folderListCacheKey(userID, videoID)

	cacheCtx, cancel := context.WithTimeout(context.Background(), folderListRedisTimeout)
	defer cancel()

	raw, err := c.redis.Get(cacheCtx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.logger.Warn("get folder list cache failed", zap.String("key", key), zap.Error(err))
		}

		return nil, false
	}

	var items []app.FolderWithCount
	if err := json.Unmarshal(raw, &items); err != nil {
		c.logger.Warn("unmarshal folder list cache failed", zap.String("key", key), zap.Error(err))
		_ = c.redis.Del(cacheCtx, key).Err()

		return nil, false
	}

	copied := make([]app.FolderWithCount, len(items))
	copy(copied, items)

	return copied, true
}

// Store 写入收藏夹列表缓存。
func (c *FolderListCache) Store(userID, videoID uint, items []app.FolderWithCount) {
	if c == nil || c.redis == nil {
		return
	}

	key := folderListCacheKey(userID, videoID)

	raw, err := json.Marshal(items)
	if err != nil {
		c.logger.Warn("marshal folder list cache failed", zap.String("key", key), zap.Error(err))

		return
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), folderListRedisTimeout)
	defer cancel()

	if err := c.redis.Set(cacheCtx, key, raw, folderListCacheTTL).Err(); err != nil {
		c.logger.Warn("set folder list cache failed", zap.String("key", key), zap.Error(err))
	}
}

// Invalidate 按用户前缀清理全部收藏夹列表缓存（含各 videoId 变体）。
func (c *FolderListCache) Invalidate(userID uint) {
	if c == nil || c.redis == nil || userID == 0 {
		return
	}

	prefix := constants.FolderListCacheKeyPrefix + strconv.FormatUint(uint64(userID), 10) + ":"

	cacheCtx, cancel := context.WithTimeout(context.Background(), folderListRedisTimeout)
	defer cancel()

	if err := c.deleteKeysByPrefix(cacheCtx, prefix); err != nil {
		c.logger.Warn("invalidate folder list cache failed", zap.Uint("userId", userID), zap.Error(err))
	}
}

// deleteKeysByPrefix 用 SCAN 游标批量删除前缀匹配的 key（避免 KEYS 阻塞）。
func (c *FolderListCache) deleteKeysByPrefix(ctx context.Context, prefix string) error {
	if c.redis == nil || prefix == "" {
		return nil
	}

	var cursor uint64

	pattern := prefix + "*"

	for {
		keys, next, err := c.redis.Scan(ctx, cursor, pattern, folderListScanBatch).Result()
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

// folderListCacheKey 收藏夹列表缓存 key。
func folderListCacheKey(userID, videoID uint) string {
	return constants.FolderListCacheKeyPrefix +
		strconv.FormatUint(uint64(userID), 10) + ":" +
		strconv.FormatUint(uint64(videoID), 10)
}
