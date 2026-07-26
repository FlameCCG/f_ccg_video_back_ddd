package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	app "f_ccg_video_back_ddd/internal/application/platform"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// loadVideoStatsSnapshotScript 对每个 video 原子读取 stats + version，避免 MGET 交叉快照。
// KEYS 布局：[statsKey1, versionKey1, statsKey2, versionKey2, ...]
// 返回扁平数组：[stats1, version1, stats2, version2, ...]
var loadVideoStatsSnapshotScript = redis.NewScript(`
local result = {}
for i = 1, #KEYS, 2 do
  result[#result + 1] = redis.call("GET", KEYS[i])
  result[#result + 1] = redis.call("GET", KEYS[i + 1])
end
return result
`)

// ackVideoStatsVersionScript 仅当版本号与落库前快照一致时才摘除脏标记。
var ackVideoStatsVersionScript = redis.NewScript(`
local dirtyKey = KEYS[1]
local versionKey = KEYS[2]
local videoId = ARGV[1]
local expected = tonumber(ARGV[2] or "0") or 0
local current = tonumber(redis.call("GET", versionKey) or "0") or 0

if current == expected then
  redis.call("SREM", dirtyKey, videoId)
  return 1
end

return 0
`)

// videoStatsPayload 视频统计缓存的 JSON 载荷。
//
// json tag 是缓存的外部契约，与写入端（video 上下文）逐字一致。
type videoStatsPayload struct {
	Views         int64 `json:"views"`
	Likes         int64 `json:"likes"`
	CommentCount  int64 `json:"commentCount"`
	CoinCount     int64 `json:"coinCount"`
	FavoriteCount int64 `json:"favoriteCount"`
	DanmuCount    int64 `json:"danmuCount"`
}

// VideoStatsCache 视频统计缓存的 Redis 实现。
//
// Redis key 与原实现完全一致（脏集合 video:detail:stats:dirty、
// 统计 video:detail:stats:<id>、版本 video:detail:stats:ver:<id>）。
type VideoStatsCache struct {
	redis *redis.Client
}

var _ app.VideoStatsCache = (*VideoStatsCache)(nil)

// NewVideoStatsCache 构造视频统计缓存适配器。
func NewVideoStatsCache(redisClient *redis.Client) *VideoStatsCache {
	return &VideoStatsCache{redis: redisClient}
}

// Available 报告底层 Redis 是否已配置。
func (c *VideoStatsCache) Available() bool {
	return c.redis != nil
}

// ScanDirty 游标扫描脏视频集合（SSCAN）。
func (c *VideoStatsCache) ScanDirty(ctx context.Context, cursor uint64, count int64) ([]string, uint64, error) {
	res, nextCursor, err := c.redis.SScan(ctx, constants.VideoDetailStatsDirtySetKey, cursor, "*", count).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, 0, err
	}

	return res, nextCursor, nil
}

// LoadSnapshots 用 Lua 原子读取每个成员的统计值与版本号。
//
// 成员非法或统计值缺失时直接摘除脏标记并跳过；
// 快照数组不完整或反序列化失败时仅告警跳过，保留脏标记等待下轮。
func (c *VideoStatsCache) LoadSnapshots(ctx context.Context, ids []string) ([]app.VideoStatsSnapshot, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// 每个 id 一对 key：stats + version，由 Lua 原子读取同一快照。
	keys := make([]string, 0, len(ids)*2)
	for _, id := range ids {
		keys = append(keys, constants.VideoDetailStatsKeyPrefix+id)
		keys = append(keys, constants.VideoDetailStatsVersionKeyPrefix+id)
	}

	rawResult, err := loadVideoStatsSnapshotScript.Run(ctx, c.redis, keys).Slice()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	// 期望 2 * len(ids) 个元素；不足时按已有结果解析，避免越界。
	snapshots := make([]app.VideoStatsSnapshot, 0, len(ids))

	for i, memberID := range ids {
		videoID, parseErr := parseDirtyVideoID(memberID)
		if parseErr != nil {
			zap.L().Warn("invalid dirty video id", zap.String("member", memberID), zap.Error(parseErr))
			c.clearDirtyMember(ctx, memberID)

			continue
		}

		statsIdx := i * 2
		versionIdx := statsIdx + 1

		if versionIdx >= len(rawResult) {
			zap.L().Warn("load dirty video stats snapshot incomplete", zap.String("member", memberID))

			continue
		}

		raw := bytesFromRedisValue(rawResult[statsIdx])
		if len(raw) == 0 {
			c.clearDirtyMember(ctx, memberID)

			continue
		}

		var payload videoStatsPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			zap.L().Warn("unmarshal dirty video stats failed", zap.Uint("videoId", videoID), zap.Error(err))

			continue
		}

		stats := app.VideoStats{
			Views:         payload.Views,
			Likes:         payload.Likes,
			CommentCount:  payload.CommentCount,
			CoinCount:     payload.CoinCount,
			FavoriteCount: payload.FavoriteCount,
			DanmuCount:    payload.DanmuCount,
		}

		snapshots = append(snapshots, app.VideoStatsSnapshot{
			VideoID:  videoID,
			Version:  int64FromRedisValue(rawResult[versionIdx]),
			Stats:    &stats,
			MemberID: memberID,
		})
	}

	return snapshots, nil
}

// AckVersion 版本未变化时摘除脏标记；版本已变化则保留，等待下轮同步。
func (c *VideoStatsCache) AckVersion(ctx context.Context, memberID string, version int64) error {
	_, err := ackVideoStatsVersionScript.Run(ctx, c.redis, []string{
		constants.VideoDetailStatsDirtySetKey,
		constants.VideoDetailStatsVersionKeyPrefix + memberID,
	}, memberID, strconv.FormatInt(version, 10)).Result()

	return err
}

// clearDirtyMember 丢弃无法处理的脏成员：移出脏集合并删除其版本键。
func (c *VideoStatsCache) clearDirtyMember(ctx context.Context, memberID string) {
	if memberID == "" {
		return
	}

	_ = c.redis.SRem(ctx, constants.VideoDetailStatsDirtySetKey, memberID).Err()
	_ = c.redis.Del(ctx, constants.VideoDetailStatsVersionKeyPrefix+memberID).Err()
}

// parseDirtyVideoID 解析脏集合成员为视频 ID。
func parseDirtyVideoID(raw string) (uint, error) {
	val, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, err
	}

	return uint(val), nil
}
