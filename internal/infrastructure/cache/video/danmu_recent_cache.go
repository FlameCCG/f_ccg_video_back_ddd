package video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// danmuCacheTimeout 最近弹幕缓存的单次操作超时（与原实现的 2s 一致）。
const danmuCacheTimeout = 2 * time.Second

// DanmuRecentCache 最近弹幕列表的 Redis 适配器
// （原 service/video_service/danmu_service.go 的 cacheRecentDanmu / invalidateDanmuRecentCache）。
type DanmuRecentCache struct {
	redis  *redis.Client
	logger *zap.Logger
}

var _ app.DanmuRecentCache = (*DanmuRecentCache)(nil)

// NewDanmuRecentCache 构造最近弹幕缓存适配器；redisClient 允许为 nil。
func NewDanmuRecentCache(redisClient *redis.Client, logger *zap.Logger) *DanmuRecentCache {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &DanmuRecentCache{redis: redisClient, logger: logger}
}

// Available 报告 Redis 是否可用。
func (c *DanmuRecentCache) Available() bool {
	return c != nil && c.redis != nil
}

// Push 追加一条弹幕并裁剪列表长度、续期 TTL（单个事务型 pipeline）。
func (c *DanmuRecentCache) Push(ctx context.Context, dto *app.DanmuDTO) {
	if !c.Available() || dto == nil {
		return
	}

	data, err := json.Marshal(dto)
	if err != nil {
		c.logger.Warn("marshal danmu cache failed", zap.Error(err), zap.Uint("videoId", dto.VideoID))

		return
	}

	key := danmuRecentKey(dto.VideoID, dto.VideoPartID)

	cacheCtx, cancel := context.WithTimeout(orTODO(ctx), danmuCacheTimeout)
	defer cancel()

	pipe := c.redis.TxPipeline()
	pipe.RPush(cacheCtx, key, data)
	pipe.LTrim(cacheCtx, key, -domain.DanmuRecentMax, -1)
	pipe.Expire(cacheCtx, key, domain.DanmuRecentCacheTTL)

	if _, err := pipe.Exec(cacheCtx); err != nil {
		c.logger.Warn("write danmu cache failed", zap.Error(err), zap.Uint("videoId", dto.VideoID))
	}
}

// Invalidate 删除某房间的最近弹幕缓存。
func (c *DanmuRecentCache) Invalidate(ctx context.Context, videoID, partID uint) {
	if !c.Available() || videoID == 0 {
		return
	}

	cacheCtx, cancel := context.WithTimeout(orTODO(ctx), danmuCacheTimeout)
	defer cancel()

	key := danmuRecentKey(videoID, partID)
	if err := c.redis.Del(cacheCtx, key).Err(); err != nil && !errors.Is(err, redis.Nil) {
		c.logger.Warn("clear recent danmu cache failed",
			zap.Error(err), zap.Uint("videoId", videoID), zap.Uint("partId", partID))
	}
}

// danmuRecentKey 最近弹幕列表 key。
func danmuRecentKey(videoID, partID uint) string {
	return fmt.Sprintf("%s%d:%d", constants.DanmuRecentKeyPrefix, videoID, partID)
}
