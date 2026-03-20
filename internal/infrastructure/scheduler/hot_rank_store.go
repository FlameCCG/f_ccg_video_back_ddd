package scheduler

import (
	"context"
	"errors"
	"fmt"

	app "f_ccg_video_back_ddd/internal/application/platform"
	platformservice "f_ccg_video_back_ddd/internal/domain/platform/service"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
)

// HotRankStore 使用 Redis key constants.VideoHotRankZSetKey 保存热门榜单。
type HotRankStore struct {
	redis *redis.Client
}

var _ app.HotRankStore = (*HotRankStore)(nil)

// NewHotRankStore 构造热门榜单存储适配器。
func NewHotRankStore(redisClient *redis.Client) *HotRankStore {
	return &HotRankStore{redis: redisClient}
}

// Available 报告底层 Redis 是否已配置。
func (s *HotRankStore) Available() bool {
	return s.redis != nil
}

// Size 返回榜单当前成员数（ZCARD）。
func (s *HotRankStore) Size(ctx context.Context) (int64, error) {
	total, err := s.redis.ZCard(ctx, constants.VideoHotRankZSetKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, err
	}

	return total, nil
}

// Entries 返回榜单全部成员及分值（ZRANGE 0 -1 WITHSCORES）。
func (s *HotRankStore) Entries(ctx context.Context) ([]platformservice.HotRankEntry, error) {
	members, err := s.redis.ZRangeWithScores(ctx, constants.VideoHotRankZSetKey, 0, -1).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	entries := make([]platformservice.HotRankEntry, 0, len(members))
	for _, member := range members {
		entries = append(entries, platformservice.HotRankEntry{
			// go-redis 返回的成员类型为 any，使用 fmt.Sprint 归一化为字符串。
			Member: fmt.Sprint(member.Member),
			Score:  member.Score,
		})
	}

	return entries, nil
}

// ApplyDecay 在同一管线内按入参顺序写回新分值或移出成员。
func (s *HotRankStore) ApplyDecay(ctx context.Context, updates []platformservice.HotRankUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	pipe := s.redis.Pipeline()

	for _, update := range updates {
		if update.Remove {
			pipe.ZRem(ctx, constants.VideoHotRankZSetKey, update.Member)

			continue
		}

		pipe.ZAdd(ctx, constants.VideoHotRankZSetKey, redis.Z{
			Score:  update.Score,
			Member: update.Member,
		})
	}

	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return err
	}

	return nil
}

// TrimLowest 移除分值最低的 count 个成员（ZREMRANGEBYRANK 0 count-1）。
func (s *HotRankStore) TrimLowest(ctx context.Context, count int64) error {
	if count <= 0 {
		return nil
	}

	if err := s.redis.ZRemRangeByRank(ctx, constants.VideoHotRankZSetKey, 0, count-1).Err(); err != nil &&
		!errors.Is(err, redis.Nil) {
		return err
	}

	return nil
}
