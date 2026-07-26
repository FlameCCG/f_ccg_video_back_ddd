package video

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/video"
)

const (
	defaultFeedPageSize = 11
	maxFeedPageSize     = 50
	// maxHistoryCount 首页去重历史窗口；过大 NOT IN 会拖垮 MySQL，控制在近期观看即可。
	maxHistoryCount = 100
)

// FeedService 首页流 / 综合热门 / 排行榜用例服务
// （原 service/video_service/feed_service.go 与 hot_rank_service.go）。
type FeedService struct {
	query   FeedQuery
	history domain.HistoryRepository
	hotRank HotRankCache
	stats   *StatsService
}

// NewFeedService 构造首页流用例服务。
func NewFeedService(
	query FeedQuery,
	history domain.HistoryRepository,
	hotRank HotRankCache,
	stats *StatsService,
) *FeedService {
	return &FeedService{query: query, history: history, hotRank: hotRank, stats: stats}
}

// HomepageFeed 根据观看历史推荐首页视频，可按分区筛选。
func (s *FeedService) HomepageFeed(
	ctx context.Context,
	userID, partitionID uint,
	page, pageSize int,
) ([]FeedItem, int64, error) {
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 {
		pageSize = defaultFeedPageSize
	}

	if pageSize > maxFeedPageSize {
		pageSize = maxFeedPageSize
	}

	historyVideoIDs := s.history.RecentVideoIDs(ctx, userID, maxHistoryCount)
	preferredPartitions := s.query.PreferredPartitionIDs(ctx, userID)

	var (
		items []FeedItem
		total int64
		err   error
	)

	if partitionID > 0 {
		filtered := []uint{partitionID}

		items, total, err = s.query.FetchFeed(ctx, filtered, historyVideoIDs, page, pageSize)
		if err != nil {
			return nil, 0, err
		}

		if total == 0 && len(historyVideoIDs) > 0 {
			items, total, err = s.query.FetchFeed(ctx, filtered, nil, page, pageSize)
			if err != nil {
				return nil, 0, err
			}
		}
	} else {
		items, total, err = s.query.FetchFeed(ctx, preferredPartitions, historyVideoIDs, page, pageSize)
		if err != nil {
			return nil, 0, err
		}

		// 若根据偏好分区无结果，则退化为全局热门；若历史过少导致空集，进一步放开去重
		if total == 0 {
			items, total, err = s.query.FetchFeed(ctx, nil, historyVideoIDs, page, pageSize)
			if err != nil {
				return nil, 0, err
			}
		}

		if total == 0 && len(historyVideoIDs) > 0 {
			items, total, err = s.query.FetchFeed(ctx, nil, nil, page, pageSize)
			if err != nil {
				return nil, 0, err
			}
		}
	}

	return s.applyCachedStats(ctx, items), total, nil
}

// applyCachedStats 用 Redis 权威计数覆盖列表项的播放量与弹幕数。
func (s *FeedService) applyCachedStats(ctx context.Context, items []FeedItem) []FeedItem {
	if len(items) == 0 {
		return []FeedItem{}
	}

	ids := make([]uint, 0, len(items))
	for i := range items {
		if items[i].ID > 0 {
			ids = append(ids, items[i].ID)
		}
	}

	statsMap := s.stats.LoadBatch(ctx, ids)

	out := make([]FeedItem, len(items))
	for i := range items {
		out[i] = applyCachedStatsToFeedItem(items[i], statsMap[items[i].ID])
	}

	return out
}
