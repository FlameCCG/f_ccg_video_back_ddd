package video

import (
	"context"
)

// RecommendQuery 相关推荐候选集查询端口。
type RecommendQuery interface {
	// BaseVideo 读取推荐基准视频的分区；视频不存在或未发布时返回 domain.ErrVideoNotPublished。
	BaseVideo(ctx context.Context, videoID uint) (partitionID uint, err error)
	// TagIDs 读取视频的标签 ID 列表。
	TagIDs(ctx context.Context, videoID uint) []uint
	// TagRelatedIDs 按共同标签数排序取候选视频 ID。
	TagRelatedIDs(ctx context.Context, videoID uint, tagIDs []uint, limit int) ([]uint, error)
	// PartitionRelatedIDs 按同分区热度取候选视频 ID。
	PartitionRelatedIDs(ctx context.Context, partitionID uint, limit int, exclude []uint) ([]uint, error)
	// HotRelatedIDs 按全局热度取候选视频 ID。
	HotRelatedIDs(ctx context.Context, limit int, exclude []uint) ([]uint, error)
	// FetchAnyStatusByIDs 按 ID 列表读取列表项（不过滤状态），返回 id → item 映射。
	FetchAnyStatusByIDs(ctx context.Context, videoIDs []uint) (map[uint]FeedItem, error)
}

// VideoSearcher 视频搜索端口。
//
// 绑定目标：search 上下文。
// 返回值使用 any，以保证响应 JSON 完全等于搜索上下文产出的结构。
type VideoSearcher interface {
	SearchVideos(ctx context.Context, opts VideoSearchOptions) (any, error)
	Suggest(ctx context.Context, prefix string, size int) (any, error)
	HotKeywords(ctx context.Context, page, pageSize int) (any, int64, error)
	RecordHotKeyword(ctx context.Context, keyword string)
}

// VideoSearchOptions 视频搜索参数。
type VideoSearchOptions struct {
	Keyword    string
	Page       int
	PageSize   int
	VideoSort  int
	VideoOrder int
}
