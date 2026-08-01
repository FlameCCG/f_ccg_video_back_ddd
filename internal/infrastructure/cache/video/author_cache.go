package video

import (
	"context"
	"strconv"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"

	"github.com/redis/go-redis/v9"
)

const (
	// videoAuthorIDCacheKeyPrefix 视频作者 ID 缓存 key 前缀。
	videoAuthorIDCacheKeyPrefix = "video:author:id:"
	// videoAuthorIDCacheTTL 作者 ID 使用长 TTL，变更路径必须主动失效；禁止永久 key。
	videoAuthorIDCacheTTL = 30 * 24 * time.Hour
)

// AuthorIDCache 视频作者 ID 的 Redis 适配器（原 service/video_service/author_cache.go）。
type AuthorIDCache struct {
	redis *redis.Client
}

var _ app.AuthorIDCache = (*AuthorIDCache)(nil)

// NewAuthorIDCache 构造作者 ID 缓存适配器；redisClient 允许为 nil。
func NewAuthorIDCache(redisClient *redis.Client) *AuthorIDCache {
	return &AuthorIDCache{redis: redisClient}
}

// Available 报告 Redis 是否可用。
func (c *AuthorIDCache) Available() bool {
	return c != nil && c.redis != nil
}

// Load 读取作者 ID；未命中返回 (0, false)。
func (c *AuthorIDCache) Load(ctx context.Context, videoID uint) (uint, bool) {
	if !c.Available() {
		return 0, false
	}

	val, err := c.redis.Get(orTODO(ctx), authorIDKey(videoID)).Uint64()
	if err != nil || val == 0 {
		return 0, false
	}

	return uint(val), true
}

// Store 写入作者 ID（长 TTL）。
func (c *AuthorIDCache) Store(ctx context.Context, videoID, authorID uint) {
	if !c.Available() || videoID == 0 || authorID == 0 {
		return
	}

	_ = c.redis.Set(
		orTODO(ctx),
		authorIDKey(videoID),
		strconv.FormatUint(uint64(authorID), 10),
		videoAuthorIDCacheTTL,
	).Err()
}

// Invalidate 删除作者 ID 缓存。
func (c *AuthorIDCache) Invalidate(ctx context.Context, videoID uint) {
	if !c.Available() || videoID == 0 {
		return
	}

	_ = c.redis.Del(orTODO(ctx), authorIDKey(videoID)).Err()
}

// authorIDKey 视频作者 ID 缓存 key。
func authorIDKey(videoID uint) string {
	return videoAuthorIDCacheKeyPrefix + strconv.FormatUint(uint64(videoID), 10)
}
