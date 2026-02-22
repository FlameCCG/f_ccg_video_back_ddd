package media

import (
	"context"
	"strconv"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/media"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// videoDetailCacheUpdateTimeout 详情缓存失效操作的最长等待。
	videoDetailCacheUpdateTimeout = 2 * time.Second
	// videoDetailMetaGenTTL 与 video 上下文的 videoDetailMetaGenTTL 对齐：
	// 略长于 meta TTL，阻止在途旧快照回填。
	videoDetailMetaGenTTL = 10 * time.Minute
)

// VideoDetailCache 视频详情 meta 缓存失效适配器。
//
// 转码或资源变更后删除 meta 并递增 generation，由读路径重建；
// generation 阻止在途数据库快照回填失效数据。不能用 GET→patch→SetXX 更新整个缓存对象。
// 这些 key 归 video 上下文所有。
type VideoDetailCache struct {
	redis  *redis.Client
	logger *zap.Logger
}

var _ domain.VideoDetailCacheInvalidator = (*VideoDetailCache)(nil)

// NewVideoDetailCache 构造视频详情缓存失效适配器。
func NewVideoDetailCache(redisClient *redis.Client, logger *zap.Logger) *VideoDetailCache {
	return &VideoDetailCache{redis: redisClient, logger: logger}
}

// InvalidateVideoDetailMeta 删除详情 meta 缓存并 bump generation。
//
// partID 仅保留签名兼容：整 key 失效不依赖分P 粒度 patch。
func (c *VideoDetailCache) InvalidateVideoDetailMeta(ctx context.Context, videoID, _ uint) {
	if c == nil || c.redis == nil || videoID == 0 {
		return
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	cacheCtx, cancel := context.WithTimeout(ctx, videoDetailCacheUpdateTimeout)
	defer cancel()

	id := strconv.FormatUint(uint64(videoID), 10)
	metaKey := constants.VideoDetailCacheKeyPrefix + id
	genKey := constants.VideoDetailMetaGenKeyPrefix + id

	pipe := c.redis.Pipeline()
	pipe.Del(cacheCtx, metaKey)
	pipe.Incr(cacheCtx, genKey)
	pipe.Expire(cacheCtx, genKey, videoDetailMetaGenTTL)

	if _, err := pipe.Exec(cacheCtx); err != nil {
		c.logger.Warn("invalidate video detail meta cache failed",
			zap.Error(err),
			zap.Uint("videoId", videoID),
		)
	}
}
