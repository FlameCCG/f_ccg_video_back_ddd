// Package video 适配视频统计、详情元数据、上传配额与并发锁。
//
// stats_cache.go 管理计数和版本，detail_meta_cache.go 管理可失效的详情快照；两者不能混成同一份静态缓存。
// upload_quota.go 用 Lua 做日窗口与哈希去重，upload_guard.go 用带令牌的锁协调投稿和清理。
// 还有作者、收藏夹、分区、热榜和弹幕相关缓存/广播；定时发布时间的权威存储在 MySQL videos.publish_at。
// 阅读缓存要同时检查 key、TTL、失效与回填条件，锁的过期和所有权也属于正确性的一部分。
//
// 本包实现所属上下文声明的端口，序列化和 Redis 命令留在外层；改变 key 需考虑已存在的数据。
package video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// legacyVideoStatsSyncedKeyPrefix 历史遗留的 synced 标记 key 前缀（清理时一并删除）。
	legacyVideoStatsSyncedKeyPrefix = constants.VideoDetailStatsKeyPrefix + "synced:"
	// videoStatsCacheTTL 权威计数与 version 的滑动 TTL；增量/回填会续期，冷视频自动淘汰。
	videoStatsCacheTTL = 7 * 24 * time.Hour
	// videoDetailCacheGrace 单次 Redis 读写的宽限超时（与原 videoDetailCacheGrace 一致）。
	videoDetailCacheGrace = 2 * time.Second
)

// applyVideoStatsDeltaScript 原子更新 Redis 权威计数，并标记 dirty + 递增 version。
//
// 三个动作要放在同一脚本里：若先更新计数再单独登记 dirty，进程中断可能留下永远不回写的新值。
// version 让回写器只能确认自己读到的快照，避免清掉期间新产生的 dirty；它不负责 SQL/Redis 跨系统原子性。
var applyVideoStatsDeltaScript = redis.NewScript(`
local statsKey = KEYS[1]
local dirtyKey = KEYS[2]
local versionKey = KEYS[3]
local videoId = ARGV[1]
local base = ARGV[2]

local raw = redis.call("GET", statsKey)
if (not raw or raw == false or raw == "") and base and base ~= "" then
  raw = base
end
if not raw or raw == false or raw == "" then
  raw = '{"views":0,"likes":0,"commentCount":0,"coinCount":0,"favoriteCount":0,"danmuCount":0}'
end

local stats = cjson.decode(raw)

local function bump(field, delta)
  local n = tonumber(delta or "0") or 0
  if n == 0 then
    return
  end
  local current = tonumber(stats[field] or 0) or 0
  current = current + n
  if current < 0 then
    current = 0
  end
  stats[field] = current
end

bump("views", ARGV[3])
bump("likes", ARGV[4])
bump("commentCount", ARGV[5])
bump("coinCount", ARGV[6])
bump("favoriteCount", ARGV[7])
bump("danmuCount", ARGV[8])

local encoded = cjson.encode(stats)
local ttl = tonumber(ARGV[9]) or 604800
redis.call("SET", statsKey, encoded, "EX", ttl)
redis.call("INCR", versionKey)
redis.call("EXPIRE", versionKey, ttl)
redis.call("SADD", dirtyKey, videoId)

return encoded
`)

// applyVideoStatsDeltaIfExistsScript 仅在 stats key 已存在时 patch，
// 避免冷缓存从已减 DB 再 delta（双减）。
var applyVideoStatsDeltaIfExistsScript = redis.NewScript(`
local statsKey = KEYS[1]
local dirtyKey = KEYS[2]
local versionKey = KEYS[3]
local videoId = ARGV[1]

local raw = redis.call("GET", statsKey)
if not raw or raw == false or raw == "" then
  return nil
end

local stats = cjson.decode(raw)

local function bump(field, delta)
  local n = tonumber(delta or "0") or 0
  if n == 0 then
    return
  end
  local current = tonumber(stats[field] or 0) or 0
  current = current + n
  if current < 0 then
    current = 0
  end
  stats[field] = current
end

bump("views", ARGV[2])
bump("likes", ARGV[3])
bump("commentCount", ARGV[4])
bump("coinCount", ARGV[5])
bump("favoriteCount", ARGV[6])
bump("danmuCount", ARGV[7])

local encoded = cjson.encode(stats)
local ttl = tonumber(ARGV[8]) or 604800
redis.call("SET", statsKey, encoded, "EX", ttl)
redis.call("INCR", versionKey)
redis.call("EXPIRE", versionKey, ttl)
redis.call("SADD", dirtyKey, videoId)

return encoded
`)

// StatsCache 视频统计权威计数的 Redis 适配器（原 service/video_service/stats_cache.go）。
type StatsCache struct {
	redis  *redis.Client
	logger *zap.Logger
}

var _ app.StatsCache = (*StatsCache)(nil)

// NewStatsCache 构造视频统计缓存适配器；redisClient 允许为 nil（整体降级为直写 DB）。
func NewStatsCache(redisClient *redis.Client, logger *zap.Logger) *StatsCache {
	return &StatsCache{redis: redisClient, logger: logger}
}

// Available 检查 Redis 句柄是否配置；缺省时应用层直接写 DB，实际网络故障另走错误分支。
func (c *StatsCache) Available() bool {
	return c != nil && c.redis != nil
}

// Load 读取权威计数；未命中返回 (nil, false)。
func (c *StatsCache) Load(ctx context.Context, videoID uint) (*domain.Stats, bool) {
	if !c.Available() {
		return nil, false
	}

	cacheCtx, cancel := context.WithTimeout(orBackground(ctx), videoDetailCacheGrace)
	defer cancel()

	key := statsKey(videoID)

	data, err := c.redis.Get(cacheCtx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.logger.Warn("get video stats cache failed", zap.Error(err), zap.Uint("videoId", videoID))
		}

		return nil, false
	}

	var stats domain.Stats
	if err := json.Unmarshal(data, &stats); err != nil {
		c.logger.Warn("unmarshal video stats cache failed", zap.Error(err), zap.Uint("videoId", videoID))

		_ = c.redis.Del(cacheCtx, key).Err()

		return nil, false
	}

	return &stats, true
}

// LoadBatch 批量读取权威计数（MGET），缺失项不出现在结果里。
func (c *StatsCache) LoadBatch(ctx context.Context, videoIDs []uint) map[uint]*domain.Stats {
	if !c.Available() || len(videoIDs) == 0 {
		return nil
	}

	uniqueIDs := make([]uint, 0, len(videoIDs))
	seen := make(map[uint]struct{}, len(videoIDs))

	for _, id := range videoIDs {
		if id == 0 {
			continue
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}

	if len(uniqueIDs) == 0 {
		return nil
	}

	keys := make([]string, 0, len(uniqueIDs))
	for _, id := range uniqueIDs {
		keys = append(keys, statsKey(id))
	}

	values, err := c.redis.MGet(orTODO(ctx), keys...).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil
	}

	out := make(map[uint]*domain.Stats, len(uniqueIDs))

	for i, raw := range values {
		if raw == nil {
			continue
		}

		var data []byte

		switch v := raw.(type) {
		case string:
			data = []byte(v)
		case []byte:
			data = v
		default:
			continue
		}

		var stats domain.Stats
		if err := json.Unmarshal(data, &stats); err != nil {
			continue
		}

		out[uniqueIDs[i]] = &stats
	}

	return out
}

// Seed 仅在缺失时回填计数（SETNX + TTL）。
func (c *StatsCache) Seed(ctx context.Context, videoID uint, stats *domain.Stats) {
	if !c.Available() || stats == nil {
		return
	}

	cacheCtx, cancel := context.WithTimeout(orBackground(ctx), videoDetailCacheGrace)
	defer cancel()

	data, err := json.Marshal(stats)
	if err != nil {
		c.logger.Warn("marshal video stats cache failed", zap.Error(err), zap.Uint("videoId", videoID))

		return
	}

	// 数字信息优先由增量脚本维护，这里只在缺失时回填；必须带 TTL，避免永久 key 堆积。
	if err := c.redis.SetNX(cacheCtx, statsKey(videoID), data, videoStatsCacheTTL).Err(); err != nil {
		c.logger.Warn("seed video stats cache failed", zap.Error(err), zap.Uint("videoId", videoID))
	}
}

// ApplyDelta 原子应用增量并标记 dirty + 递增 version；basePayload 为冷缓存时的基线 payload。
func (c *StatsCache) ApplyDelta(
	ctx context.Context,
	videoID uint,
	delta domain.StatsDelta,
	basePayload string,
) (*domain.Stats, error) {
	if !c.Available() {
		return nil, nil
	}

	res, err := applyVideoStatsDeltaScript.Run(orTODO(ctx), c.redis, []string{
		statsKey(videoID),
		constants.VideoDetailStatsDirtySetKey,
		statsVersionKey(videoID),
	}, strconv.FormatUint(uint64(videoID), 10), basePayload,
		strconv.FormatInt(delta.Views, 10),
		strconv.FormatInt(delta.Likes, 10),
		strconv.FormatInt(delta.CommentCount, 10),
		strconv.FormatInt(delta.CoinCount, 10),
		strconv.FormatInt(delta.FavoriteCount, 10),
		strconv.FormatInt(delta.DanmuCount, 10),
		strconv.FormatInt(int64(videoStatsCacheTTL/time.Second), 10),
	).Text()
	if err != nil {
		return nil, err
	}

	var stats domain.Stats
	if unmarshalErr := json.Unmarshal([]byte(res), &stats); unmarshalErr != nil {
		// 返回 (nil, nil) 让应用层按「Lua 返回值无法解析」重试。
		return nil, nil
	}

	return &stats, nil
}

// ApplyDeltaIfExists 仅在 key 已存在时应用增量；key 不存在返回 (nil, false, nil)。
func (c *StatsCache) ApplyDeltaIfExists(
	ctx context.Context,
	videoID uint,
	delta domain.StatsDelta,
) (*domain.Stats, bool, error) {
	if !c.Available() {
		return nil, false, nil
	}

	res, err := applyVideoStatsDeltaIfExistsScript.Run(orTODO(ctx), c.redis, []string{
		statsKey(videoID),
		constants.VideoDetailStatsDirtySetKey,
		statsVersionKey(videoID),
	}, strconv.FormatUint(uint64(videoID), 10),
		strconv.FormatInt(delta.Views, 10),
		strconv.FormatInt(delta.Likes, 10),
		strconv.FormatInt(delta.CommentCount, 10),
		strconv.FormatInt(delta.CoinCount, 10),
		strconv.FormatInt(delta.FavoriteCount, 10),
		strconv.FormatInt(delta.DanmuCount, 10),
		strconv.FormatInt(int64(videoStatsCacheTTL/time.Second), 10),
	).Text()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// 冷缓存：key 不存在。
			return nil, false, nil
		}

		return nil, false, err
	}

	var stats domain.Stats
	if unmarshalErr := json.Unmarshal([]byte(res), &stats); unmarshalErr != nil {
		return nil, true, nil
	}

	return &stats, true, nil
}

// RawPayload 读取当前 stats key 的原始字符串；不存在返回 ("", false, nil)。
func (c *StatsCache) RawPayload(ctx context.Context, videoID uint) (string, bool, error) {
	if !c.Available() {
		return "", false, nil
	}

	raw, err := c.redis.Get(orTODO(ctx), statsKey(videoID)).Bytes()
	if err == nil {
		return string(raw), true, nil
	}

	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}

	return "", false, err
}

// Clear 删除 stats / version / legacy synced key 并移出 dirty 集合。
func (c *StatsCache) Clear(ctx context.Context, videoIDs ...uint) {
	if !c.Available() || len(videoIDs) == 0 {
		return
	}

	cacheCtx := orTODO(ctx)

	keys := make([]string, 0, len(videoIDs)*3)
	members := make([]any, 0, len(videoIDs))

	for _, videoID := range videoIDs {
		if videoID == 0 {
			continue
		}

		keys = append(keys,
			statsKey(videoID),
			statsVersionKey(videoID),
			legacyStatsSyncedKey(videoID),
		)
		members = append(members, strconv.FormatUint(uint64(videoID), 10))
	}

	if len(keys) > 0 {
		_ = c.redis.Del(cacheCtx, keys...).Err()
	}

	if len(members) > 0 {
		_ = c.redis.SRem(cacheCtx, constants.VideoDetailStatsDirtySetKey, members...).Err()
	}
}

// IsTransientError 判断错误是否为 Redis 瞬时错误（可重试）。
func (c *StatsCache) IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	// go-redis 常见瞬时错误文案（与原 isRedisTransientError 的候选串完全一致）。
	msg := err.Error()

	for _, part := range []string{
		"i/o timeout", "connection refused", "broken pipe", "EOF", "LOADING", "BUSY", "TRYAGAIN",
	} {
		if strings.Contains(msg, part) {
			return true
		}
	}

	return false
}

// statsKey 视频统计 key。
func statsKey(videoID uint) string {
	return fmt.Sprintf("%s%d", constants.VideoDetailStatsKeyPrefix, videoID)
}

// statsVersionKey 视频统计版本号 key。
func statsVersionKey(videoID uint) string {
	return fmt.Sprintf("%s%d", constants.VideoDetailStatsVersionKeyPrefix, videoID)
}

// legacyStatsSyncedKey 历史遗留 synced 标记 key。
func legacyStatsSyncedKey(videoID uint) string {
	return fmt.Sprintf("%s%d", legacyVideoStatsSyncedKeyPrefix, videoID)
}

// orTODO 兜底 nil context。
func orTODO(ctx context.Context) context.Context {
	if ctx == nil {
		return context.TODO()
	}

	return ctx
}

// orBackground 兜底 nil context（用于需要派生超时的场景）。
func orBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}

	return ctx
}
