package video

import (
	"context"
)

// FeedQuery 首页/热门/排行/推荐的列表读模型查询端口。
type FeedQuery interface {
	// PreferredPartitionIDs 依据观看历史统计用户的偏好分区（按次数降序）。
	PreferredPartitionIDs(ctx context.Context, userID uint) ([]uint, error)
	// FetchFeed 按分区过滤 + 历史去重拉取首页流；返回列表与总数。
	FetchFeed(ctx context.Context, partitionIDs, excludeVideoIDs []uint, page, pageSize int) ([]FeedItem, int64, error)
	// FetchByIDs 按 ID 列表读取列表项（仅已发布），返回 id → item 映射。
	FetchByIDs(ctx context.Context, videoIDs []uint) (map[uint]FeedItem, error)
	// FetchHotFromDB 用库内衰减热度表达式分页拉取综合热门。
	FetchHotFromDB(ctx context.Context, page, pageSize int) ([]FeedItem, int64, error)
	// CountPublished 统计已发布视频总数。
	CountPublished(ctx context.Context) (int64, error)
	// FetchRankFromDB 用库内综合分表达式分页拉取排行榜。
	FetchRankFromDB(ctx context.Context, offset, limit int) ([]FeedItem, error)
}
