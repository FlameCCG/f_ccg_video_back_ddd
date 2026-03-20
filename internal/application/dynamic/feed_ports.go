package dynamic

import (
	"context"
)

// FeedQuery 作品流查询端口（读侧）。
type FeedQuery interface {
	// ListAuthorWorks 以 UNION ALL 联合查询作者的视频与动态，按置顶优先 + 时间倒序取行。
	ListAuthorWorks(ctx context.Context, params AuthorFeedParams) ([]FeedRow, error)
	// CountAuthorWorks 统计作者作品总数（视频数 + 动态数，按 Sources 过滤）。
	CountAuthorWorks(ctx context.Context, params AuthorFeedCountParams) (int64, error)
	// ListFollowingWorks 以 UNION ALL 联合查询浏览者本人及其关注用户的视频与动态。
	ListFollowingWorks(ctx context.Context, params FollowingFeedParams) ([]FeedRow, error)
	// CountFollowingWorks 统计浏览者本人及其关注用户的作品总数。
	CountFollowingWorks(ctx context.Context, params FollowingFeedCountParams) (int64, error)
	// CommentCountsByDynamic 按动态批量统计评论数。
	CommentCountsByDynamic(ctx context.Context, dynamicIDs []uint) (map[uint]int64, error)
	// LikedVideoIDs 返回浏览者已点赞的视频 ID 集合。
	LikedVideoIDs(ctx context.Context, viewerID uint, videoIDs []uint) (map[uint]bool, error)
	// LikedDynamicIDs 返回浏览者已点赞的动态 ID 集合。
	LikedDynamicIDs(ctx context.Context, viewerID uint, dynamicIDs []uint) (map[uint]bool, error)
}
