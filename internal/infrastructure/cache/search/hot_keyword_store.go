package search

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/pkg/constants"
	"fmt"

	app "f_ccg_video_back_ddd/internal/application/search"

	"github.com/redis/go-redis/v9"
)

// hotKeywordIncrement 单次搜索为关键词累加的热度分.
const hotKeywordIncrement = 1

// HotKeywordStore 热搜词榜的 Redis 实现。
//
// key 为 constants.SearchHotKeywordZSetKey（search:hot:keywords:zset），
// 与原实现一致：只做 ZINCRBY 累加、不设过期，榜单由运维侧决定何时重置。
type HotKeywordStore struct {
	redis *redis.Client
}

var _ app.HotKeywordStore = (*HotKeywordStore)(nil)

// NewHotKeywordStore 构造热搜词榜存储适配器（client 允许为 nil）。
func NewHotKeywordStore(client *redis.Client) *HotKeywordStore {
	return &HotKeywordStore{redis: client}
}

// Available 报告 Redis 客户端是否就绪。
func (s *HotKeywordStore) Available() bool {
	return s != nil && s.redis != nil
}

// Card 返回榜单成员总数；key 不存在时返回 0。
func (s *HotKeywordStore) Card(ctx context.Context) (int64, error) {
	if !s.Available() {
		return 0, nil
	}

	total, err := s.redis.ZCard(ctx, constants.SearchHotKeywordZSetKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, err
	}

	return total, nil
}

// RevRangeWithScores 按分数倒序取 [start,stop] 闭区间成员。
func (s *HotKeywordStore) RevRangeWithScores(ctx context.Context, start, stop int64) ([]app.HotKeywordEntry, error) {
	if !s.Available() {
		return nil, nil
	}

	members, err := s.redis.ZRevRangeWithScores(ctx, constants.SearchHotKeywordZSetKey, start, stop).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	entries := make([]app.HotKeywordEntry, 0, len(members))
	for _, member := range members {
		entries = append(entries, app.HotKeywordEntry{
			Member: fmt.Sprint(member.Member),
			Score:  member.Score,
		})
	}

	return entries, nil
}

// Incr 给关键词热度加一；失败静默忽略（热度统计不影响搜索主流程）。
func (s *HotKeywordStore) Incr(ctx context.Context, keyword string) {
	if !s.Available() {
		return
	}

	_ = s.redis.ZIncrBy(ctx, constants.SearchHotKeywordZSetKey, hotKeywordIncrement, keyword).Err()
}
