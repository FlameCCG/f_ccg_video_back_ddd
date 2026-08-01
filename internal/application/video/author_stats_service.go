package video

import (
	"context"

	"golang.org/x/sync/singleflight"
)

// AuthorStatsService 汇总作者已发布作品的播放、点赞数据。
//
// MySQL 行内计数仅作为 Redis 未命中时的回退；短 TTL 缓存与 singleflight
// 都属于服务实例，避免原实现的包级可变状态。
type AuthorStatsService struct {
	videos UserVideoQuery
	stats  *StatsService
	cache  AuthorStatsCache
	group  singleflight.Group
}

// NewAuthorStatsService 构造作者作品统计服务。
func NewAuthorStatsService(
	videos UserVideoQuery,
	stats *StatsService,
	cache AuthorStatsCache,
) *AuthorStatsService {
	return &AuthorStatsService{
		videos: videos,
		stats:  stats,
		cache:  cache,
	}
}

// AggregateByAuthor 统计作者所有已发布视频的总播放和总点赞。
//
// 与原 AggregateUserVideoStats 保持一致：空作者或查询失败返回零值，
// Redis 中存在的视频统计覆盖 MySQL 行内值。
func (s *AuthorStatsService) AggregateByAuthor(
	ctx context.Context,
	authorUsername string,
) UserVideoStats {
	if authorUsername == "" {
		return UserVideoStats{}
	}

	if cached, ok := s.cache.Load(authorUsername); ok {
		return cached
	}

	value, _, _ := s.group.Do(authorUsername, func() (any, error) {
		result := s.aggregateUncached(ctx, authorUsername)
		s.cache.Store(authorUsername, result)

		return result, nil
	})

	result, _ := value.(UserVideoStats)

	return result
}

func (s *AuthorStatsService) aggregateUncached(
	ctx context.Context,
	authorUsername string,
) UserVideoStats {
	rows, err := s.videos.AggregateAuthorStats(ctx, authorUsername)
	if err != nil || len(rows) == 0 {
		return UserVideoStats{}
	}

	videoIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		videoIDs = append(videoIDs, row.ID)
	}

	statsByVideoID := s.stats.LoadBatch(ctx, videoIDs)

	var result UserVideoStats
	for _, row := range rows {
		if cached := statsByVideoID[row.ID]; cached != nil {
			result.TotalViews += cached.Views
			result.TotalLikes += cached.Likes

			continue
		}

		result.TotalViews += row.Views
		result.TotalLikes += row.Likes
	}

	return result
}
