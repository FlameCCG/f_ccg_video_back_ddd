package video

import (
	"context"
	videodomain "f_ccg_video_back_ddd/internal/domain/video"
)

// HotFeed 综合热门视频列表（ZSET 热度，配合定时衰减）。
func (s *FeedService) HotFeed(ctx context.Context, page, pageSize int) ([]FeedItem, int64, error) {
	if !s.hotRank.Available() {
		return s.hotFeedFromDB(ctx, page, pageSize)
	}

	page, pageSize = (videodomain.ListingPolicy{}).HotPage(page, pageSize)

	total, err := s.hotRank.Card(ctx)
	if err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return s.hotFeedFromDB(ctx, page, pageSize)
	}

	start := int64((page - 1) * pageSize)

	end := start + int64(pageSize) - 1
	if start < 0 {
		start = 0
	}

	if end < start {
		end = start
	}

	ids, err := s.hotRank.RevRange(ctx, start, end)
	if err != nil {
		return nil, 0, err
	}

	if len(ids) == 0 {
		return []FeedItem{}, total, nil
	}

	items, err := s.itemsByRankedIDs(ctx, ids)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// RankTop 查询综合排行榜视频（Top100），优先通过 Redis 热度 ZSET 分页，缺失时由数据库查询 TopN。
func (s *FeedService) RankTop(ctx context.Context, page, pageSize int) ([]FeedItem, int64, error) {
	page, pageSize = (videodomain.ListingPolicy{}).RankPage(page, pageSize)

	if s.hotRank.Available() {
		if items, total, ok := s.rankTopFromZSet(ctx, page, pageSize); ok {
			return items, total, nil
		}
	}

	return s.rankTopFromDB(ctx, page, pageSize)
}

// rankTopFromZSet 使用热度 ZSET 直接分页，避免每请求 Limit 500 + 内存排序。
func (s *FeedService) rankTopFromZSet(ctx context.Context, page, pageSize int) ([]FeedItem, int64, bool) {
	card, err := s.hotRank.Card(ctx)
	if err != nil {
		return nil, 0, false
	}

	if card == 0 {
		return nil, 0, false
	}

	window := (videodomain.ListingPolicy{}).RankWindow(card, page, pageSize)
	total := window.Total
	if window.Limit == 0 {
		return []FeedItem{}, total, true
	}
	start, end := window.Offset, window.Offset+int64(window.Limit)-1

	ids, err := s.hotRank.RevRange(ctx, start, end)
	if err != nil {
		return nil, 0, false
	}

	if len(ids) == 0 {
		return []FeedItem{}, total, true
	}

	items, err := s.itemsByRankedIDs(ctx, ids)
	if err != nil {
		return nil, 0, false
	}

	return items, total, true
}

// rankTopFromDB DB 回退：只查 TopN，按库内表达式排序后分页。
func (s *FeedService) rankTopFromDB(ctx context.Context, page, pageSize int) ([]FeedItem, int64, error) {
	total, err := s.query.CountPublished(ctx)
	if err != nil {
		return nil, 0, err
	}

	window := (videodomain.ListingPolicy{}).RankWindow(total, page, pageSize)
	total = window.Total
	if window.Limit == 0 {
		return []FeedItem{}, total, nil
	}
	items, err := s.query.FetchRankFromDB(ctx, int(window.Offset), window.Limit)
	if err != nil {
		return nil, 0, err
	}

	if len(items) == 0 {
		return []FeedItem{}, total, nil
	}

	return s.applyCachedStats(ctx, items), total, nil
}

func (s *FeedService) hotFeedFromDB(ctx context.Context, page, pageSize int) ([]FeedItem, int64, error) {
	page, pageSize = (videodomain.ListingPolicy{}).HotPage(page, pageSize)

	items, total, err := s.query.FetchHotFromDB(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []FeedItem{}, 0, nil
	}

	return s.applyCachedStats(ctx, items), total, nil
}

// itemsByRankedIDs 按 ZSet 排名顺序装配列表项（仅保留仍已发布的视频）。
func (s *FeedService) itemsByRankedIDs(ctx context.Context, ids []uint) ([]FeedItem, error) {
	itemMap, err := s.query.FetchByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	if len(itemMap) == 0 {
		return []FeedItem{}, nil
	}

	statsMap := s.stats.LoadBatch(ctx, ids)

	items := make([]FeedItem, 0, len(ids))

	for _, id := range ids {
		if item, ok := itemMap[id]; ok {
			items = append(items, applyCachedStatsToFeedItem(item, statsMap[id]))
		}
	}

	return items, nil
}
