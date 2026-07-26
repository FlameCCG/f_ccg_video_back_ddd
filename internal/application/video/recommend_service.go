package video

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/video"
)

const (
	defaultRelatedSize = 12
	maxRelatedSize     = 50
)

// RecommendService 视频详情页周边推荐用例服务（原 service/video_service/recommend_service.go）。
type RecommendService struct {
	query RecommendQuery
	stats *StatsService
}

// NewRecommendService 构造推荐用例服务。
func NewRecommendService(query RecommendQuery, stats *StatsService) *RecommendService {
	return &RecommendService{query: query, stats: stats}
}

// RelatedVideos 依次按「共同标签 → 同分区热度 → 全局热度」补齐候选并返回列表。
func (s *RecommendService) RelatedVideos(ctx context.Context, videoID uint, size int) ([]FeedItem, error) {
	if videoID == 0 {
		return nil, domain.ErrVideoIDRequired
	}

	if size <= 0 {
		size = defaultRelatedSize
	}

	if size > maxRelatedSize {
		size = maxRelatedSize
	}

	partitionID, err := s.query.BaseVideo(ctx, videoID)
	if err != nil {
		return nil, err
	}

	tagIDs := s.query.TagIDs(ctx, videoID)

	ids := make([]uint, 0, size)
	seen := map[uint]struct{}{videoID: {}}

	appendUnique := func(list []uint) {
		for _, id := range list {
			if len(ids) >= size {
				return
			}

			if _, ok := seen[id]; ok {
				continue
			}

			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}

	fetchLimit := size * 2

	if len(tagIDs) > 0 {
		tagRelated, tagErr := s.query.TagRelatedIDs(ctx, videoID, tagIDs, fetchLimit)
		if tagErr != nil {
			return nil, tagErr
		}

		appendUnique(tagRelated)
	}

	if len(ids) < size && partitionID > 0 {
		partitionRelated, pErr := s.query.PartitionRelatedIDs(ctx, partitionID, fetchLimit, collectExcludeIDs(seen))
		if pErr != nil {
			return nil, pErr
		}

		appendUnique(partitionRelated)
	}

	if len(ids) < size {
		hotRelated, hErr := s.query.HotRelatedIDs(ctx, fetchLimit, collectExcludeIDs(seen))
		if hErr != nil {
			return nil, hErr
		}

		appendUnique(hotRelated)
	}

	if len(ids) == 0 {
		return []FeedItem{}, nil
	}

	itemMap, err := s.query.FetchAnyStatusByIDs(ctx, ids)
	if err != nil {
		return nil, err
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

// collectExcludeIDs 把去重集合转成排除列表（顺序不敏感，与原实现一致）。
func collectExcludeIDs(exclude map[uint]struct{}) []uint {
	if len(exclude) == 0 {
		return nil
	}

	ids := make([]uint, 0, len(exclude))
	for id := range exclude {
		if id > 0 {
			ids = append(ids, id)
		}
	}

	return ids
}
