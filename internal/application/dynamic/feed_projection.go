package dynamic

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// rowToFeedItem 把联合查询行渲染为作品流条目。
func rowToFeedItem(r FeedRow, metrics *feedMetrics) WorkFeedItem {
	item := WorkFeedItem{
		WorkType:  enum.WorkType(r.WorkType),
		CreatedAt: r.CreatedAt,
		Author: AuthorBrief{
			ID:       r.AuthorID,
			Username: r.AuthorUsername,
			Avatar:   r.AuthorAvatar,
		},
	}
	if item.WorkType == enum.WorkTypeVideo {
		item.Video = &VideoBrief{
			ID:           r.WorkID,
			Title:        r.VideoTitle,
			Description:  r.VideoDesc,
			Cover:        r.VideoCover,
			Duration:     r.VideoDuration,
			Views:        r.VideoViews,
			DanmuCount:   r.VideoDanmu,
			LikeCount:    metrics.videoLikeCounts[r.WorkID],
			CommentCount: metrics.videoCommentCounts[r.WorkID],
			IsLiked:      metrics.likedVideoIDs[r.WorkID],
			IsPinned:     r.DynIsPinned,
			PinnedAt:     r.DynPinnedAt,
		}

		return item
	}

	item.Dynamic = &DynamicBrief{
		ID:           r.WorkID,
		Content:      r.DynContent,
		ImageURL:     r.DynImageURL,
		LikeCount:    metrics.dynamicLikeCounts[r.WorkID],
		CommentCount: metrics.dynamicComments[r.WorkID],
		IsLiked:      metrics.likedDynamicIDs[r.WorkID],
		IsPinned:     r.DynIsPinned,
		PinnedAt:     r.DynPinnedAt,
	}

	return item
}
