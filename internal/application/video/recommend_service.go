package video

import (
	"context"

	videodomain "f_ccg_video_back_ddd/internal/domain/video"
)

// RecommendService 视频详情页周边推荐用例服务。
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
		return nil, videodomain.ErrVideoIDRequired
	}

	selection := videodomain.NewRelatedSelection(videoID, size)

	partitionID, err := s.query.BaseVideo(ctx, videoID)
	if err != nil {
		return nil, err
	}

	tagIDs := s.query.TagIDs(ctx, videoID)

	fetchLimit := selection.FetchLimit()

	if len(tagIDs) > 0 {
		tagRelated, tagErr := s.query.TagRelatedIDs(ctx, videoID, tagIDs, fetchLimit)
		if tagErr != nil {
			return nil, tagErr
		}

		selection.Add(tagRelated)
	}

	if selection.NeedsMore() && partitionID > 0 {
		partitionRelated, pErr := s.query.PartitionRelatedIDs(ctx, partitionID, fetchLimit, selection.ExcludedIDs())
		if pErr != nil {
			return nil, pErr
		}

		selection.Add(partitionRelated)
	}

	if selection.NeedsMore() {
		hotRelated, hErr := s.query.HotRelatedIDs(ctx, fetchLimit, selection.ExcludedIDs())
		if hErr != nil {
			return nil, hErr
		}

		selection.Add(hotRelated)
	}

	ids := selection.IDs()
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
