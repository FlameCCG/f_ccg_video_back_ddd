package video

import (
	"context"
	"encoding/json"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/constants"
	"f_ccg_video_back_ddd/pkg/logctx"
	"fmt"
	"strconv"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// videoDetailCacheTTL 详情 meta 缓存 TTL。
	videoDetailCacheTTL = 5 * time.Minute
	// videoDetailMetaGenTTL generation TTL 略长于 meta，避免 gen 先过期导致旧回填 CAS 误通过。
	videoDetailMetaGenTTL = 10 * time.Minute
)

// cacheMetaIfGenMatchScript 仅当 generation 与回填时读取的值一致时才写入 meta，
// 防止删除/改私后的在途旧 DB 快照覆盖失效结果。
//
// 协议是读 generation → 回源数据库 → Lua 比较 generation 后回填。
// 比较与写入必须原子执行；若拆成客户端 GET/SET，中间仍可能插入一次失效操作。
// 它约束回填竞争，不是 MySQL 与 Redis 的原子事务，也不替代正确的失效调用与故障处理。
var cacheMetaIfGenMatchScript = redis.NewScript(`
local genKey = KEYS[1]
local metaKey = KEYS[2]
local expected = tonumber(ARGV[1] or "0") or 0
local payload = ARGV[2]
local ttl = tonumber(ARGV[3] or "0") or 0

local current = tonumber(redis.call("GET", genKey) or "0") or 0
if current ~= expected then
  return 0
end

if ttl > 0 then
  redis.call("SET", metaKey, payload, "EX", ttl)
else
  redis.call("SET", metaKey, payload)
end
return 1
`)

// DetailMetaCache 视频详情 meta 的 Redis 适配器（generation CAS 防旧快照回填）。
type DetailMetaCache struct {
	redis  *redis.Client
	logger *zap.Logger
}

var _ app.DetailMetaCache = (*DetailMetaCache)(nil)

// NewDetailMetaCache 构造详情 meta 缓存适配器；redisClient 允许为 nil。
func NewDetailMetaCache(redisClient *redis.Client, logger *zap.Logger) *DetailMetaCache {
	return &DetailMetaCache{redis: redisClient, logger: logger}
}

// Available 报告 Redis 句柄是否已配置，不执行连通性探测。
func (c *DetailMetaCache) Available() bool {
	return c != nil && c.redis != nil
}

// Load 读取详情 meta；未命中返回 (nil, false)。
//
// 反序列化后按原实现重做三步归一化：MinIO 播放 URL 修正、资源去重排序、单分P归一化。
func (c *DetailMetaCache) Load(ctx context.Context, videoID uint) (*app.DetailMeta, bool) {
	if !c.Available() {
		return nil, false
	}

	cacheCtx, cancel := context.WithTimeout(orBackground(ctx), videoDetailCacheGrace)
	defer cancel()

	data, err := c.redis.Get(cacheCtx, metaKey(videoID)).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			logctx.With(ctx, c.logger).Warn("get video detail meta cache failed", zap.String("event", "cache.detail_meta_cache.load.degraded"), zap.String("operation", "cache.detail_meta_cache.load"), zap.Error(err), zap.Uint("video_id", videoID))
		}

		return nil, false
	}

	var meta app.DetailMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		logctx.With(ctx, c.logger).Warn("unmarshal video detail meta cache failed", zap.String("event", "cache.detail_meta_cache.load.degraded"), zap.String("operation", "cache.detail_meta_cache.load"), zap.Error(err), zap.Uint("video_id", videoID))

		return nil, false
	}

	if meta.StorageType == enum.StorageTypeMinio {
		for i := range meta.Resources {
			meta.Resources[i].FileURL = domain.EnsurePlayableContentTypeURL(meta.Resources[i].FileURL)
		}

		for i := range meta.Parts {
			for j := range meta.Parts[i].Resources {
				meta.Parts[i].Resources[j].FileURL = domain.EnsurePlayableContentTypeURL(meta.Parts[i].Resources[j].FileURL)
			}
		}
	}

	if len(meta.Resources) > 0 {
		meta.Resources = domain.DedupeResourceBriefs(meta.Resources)
		domain.SortResourceBriefs(meta.Resources)
	}

	for i := range meta.Parts {
		if len(meta.Parts[i].Resources) == 0 {
			continue
		}

		meta.Parts[i].Resources = domain.DedupeResourceBriefs(meta.Parts[i].Resources)
		domain.SortResourceBriefs(meta.Parts[i].Resources)
	}

	infra.NormalizeSinglePartDetailMeta(&meta)

	return &meta, true
}

// Generation 读取 generation 快照（必须在读库之前捕获）；key 不存在视为 0。
func (c *DetailMetaCache) Generation(ctx context.Context, videoID uint) int64 {
	if !c.Available() || videoID == 0 {
		return 0
	}

	cacheCtx, cancel := context.WithTimeout(orBackground(ctx), videoDetailCacheGrace)
	defer cancel()

	val, err := c.redis.Get(cacheCtx, metaGenKey(videoID)).Int64()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			logctx.With(ctx, c.logger).Warn("get video detail meta generation failed", zap.String("event", "cache.detail_meta_cache.generation.degraded"), zap.String("operation", "cache.detail_meta_cache.generation"),
				zap.Error(err), zap.Uint("video_id", videoID))
		}

		return 0
	}

	return val
}

// StoreIfGenerationMatch 仅在 generation 未变化时写入 meta。
func (c *DetailMetaCache) StoreIfGenerationMatch(
	ctx context.Context,
	videoID uint,
	meta *app.DetailMeta,
	expectedGen int64,
) {
	if !c.Available() || meta == nil || videoID == 0 {
		return
	}

	cacheCtx, cancel := context.WithTimeout(orBackground(ctx), videoDetailCacheGrace)
	defer cancel()

	data, err := json.Marshal(meta)
	if err != nil {
		logctx.With(ctx, c.logger).Warn("marshal video detail meta cache failed", zap.String("event", "cache.detail_meta_cache.store_if_generation_match.degraded"), zap.String("operation", "cache.detail_meta_cache.store_if_generation_match"), zap.Error(err), zap.Uint("video_id", videoID))

		return
	}

	ttlSeconds := int(videoDetailCacheTTL / time.Second)

	ok, err := cacheMetaIfGenMatchScript.Run(cacheCtx, c.redis, []string{
		metaGenKey(videoID),
		metaKey(videoID),
	}, strconv.FormatInt(expectedGen, 10), string(data), strconv.Itoa(ttlSeconds)).Int()
	if err != nil {
		logctx.With(ctx, c.logger).Warn("set video detail meta cache failed", zap.String("event", "cache.detail_meta_cache.store_if_generation_match.degraded"), zap.String("operation", "cache.detail_meta_cache.store_if_generation_match"), zap.Error(err), zap.Uint("video_id", videoID))

		return
	}

	if ok == 0 {
		logctx.With(ctx, c.logger).Debug("skip video detail meta cache write: generation changed", zap.String("event", "cache.detail_meta_cache.store_if_generation_match.diagnostic"), zap.String("operation", "cache.detail_meta_cache.store_if_generation_match"),
			zap.Uint("video_id", videoID), zap.Int64("gen", expectedGen))
	}
}

// Invalidate 删除 meta 并 bump generation。
func (c *DetailMetaCache) Invalidate(ctx context.Context, videoID uint) {
	if !c.Available() || videoID == 0 {
		return
	}

	baseCtx := orTODO(ctx)

	_ = c.redis.Del(baseCtx, metaKey(videoID)).Err()
	// 同步 bump generation，阻止在途旧 DB 快照回填。
	c.bumpGeneration(baseCtx, videoID)
}

// Delete 仅删除 meta key（不 bump generation），用于审核流程的历史行为兼容。
func (c *DetailMetaCache) Delete(ctx context.Context, videoID uint) {
	if !c.Available() || videoID == 0 {
		return
	}

	_ = c.redis.Del(orTODO(ctx), metaKey(videoID)).Err()
}

// bumpGeneration 递增 generation，阻止在途旧 DB 快照 CAS 回填成功。
func (c *DetailMetaCache) bumpGeneration(ctx context.Context, videoID uint) {
	cacheCtx, cancel := context.WithTimeout(orBackground(ctx), videoDetailCacheGrace)
	defer cancel()

	key := metaGenKey(videoID)

	pipe := c.redis.Pipeline()
	pipe.Incr(cacheCtx, key)
	pipe.Expire(cacheCtx, key, videoDetailMetaGenTTL)

	if _, err := pipe.Exec(cacheCtx); err != nil {
		logctx.With(ctx, c.logger).Warn("bump video detail meta generation failed", zap.String("event", "cache.detail_meta_cache.bump_generation.degraded"), zap.String("operation", "cache.detail_meta_cache.bump_generation"),
			zap.Error(err), zap.Uint("video_id", videoID))
	}
}

// metaKey 详情 meta key。
func metaKey(videoID uint) string {
	return fmt.Sprintf("%s%d", constants.VideoDetailCacheKeyPrefix, videoID)
}

// metaGenKey 详情 meta generation key。
func metaGenKey(videoID uint) string {
	return fmt.Sprintf("%s%d", constants.VideoDetailMetaGenKeyPrefix, videoID)
}
