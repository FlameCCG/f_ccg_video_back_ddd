package video

import (
	"context"
	"errors"
	"strconv"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
)

// HotRankCache 视频热度 ZSet 的 Redis 适配器
// （原 service/video_service/hot_score.go 与 hot_rank_service.go 的 ZSet 部分）。
type HotRankCache struct {
	redis *redis.Client
}

var _ app.HotRankCache = (*HotRankCache)(nil)

// NewHotRankCache 构造热度 ZSet 适配器；redisClient 允许为 nil。
func NewHotRankCache(redisClient *redis.Client) *HotRankCache {
	return &HotRankCache{redis: redisClient}
}

// Available 报告 Redis 是否可用。
func (c *HotRankCache) Available() bool {
	return c != nil && c.redis != nil
}

// IncrBy 累加热度分。
func (c *HotRankCache) IncrBy(ctx context.Context, videoID uint, score float64) {
	if !c.Available() || videoID == 0 || score == 0 {
		return
	}

	_ = c.redis.ZIncrBy(orTODO(ctx), constants.VideoHotRankZSetKey, score, hotScoreMember(videoID)).Err()
}

// Card 读取 ZSet 基数。
func (c *HotRankCache) Card(ctx context.Context) (int64, error) {
	if !c.Available() {
		return 0, nil
	}

	total, err := c.redis.ZCard(orTODO(ctx), constants.VideoHotRankZSetKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, err
	}

	return total, nil
}

// RevRange 倒序取排名区间的视频 ID（跳过无法解析的 member）。
func (c *HotRankCache) RevRange(ctx context.Context, start, stop int64) ([]uint, error) {
	if !c.Available() {
		return nil, nil
	}

	members, err := c.redis.ZRevRange(orTODO(ctx), constants.VideoHotRankZSetKey, start, stop).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	return parseHotVideoIDs(members), nil
}

// Remove 从 ZSet 移除视频。
func (c *HotRankCache) Remove(ctx context.Context, videoIDs []uint) {
	if !c.Available() || len(videoIDs) == 0 {
		return
	}

	members := make([]any, 0, len(videoIDs))
	for _, id := range videoIDs {
		if id == 0 {
			continue
		}

		members = append(members, id)
	}

	if len(members) == 0 {
		return
	}

	_ = c.redis.ZRem(orTODO(ctx), constants.VideoHotRankZSetKey, members...).Err()
}

// hotScoreMember ZSet 成员名（视频 ID 的十进制字符串）。
func hotScoreMember(videoID uint) string {
	return strconv.FormatUint(uint64(videoID), 10)
}

// parseHotVideoIDs 把 ZSet member 列表解析为视频 ID（跳过非法值）。
func parseHotVideoIDs(members []string) []uint {
	if len(members) == 0 {
		return nil
	}

	ids := make([]uint, 0, len(members))

	for _, m := range members {
		if m == "" {
			continue
		}

		val, err := strconv.ParseUint(m, 10, 64)
		if err != nil || val == 0 {
			continue
		}

		ids = append(ids, uint(val))
	}

	return ids
}
