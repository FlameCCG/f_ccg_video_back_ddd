package identity

import (
	"context"
)

// 用户主页「最近动作」类型（取值必须与 video 上下文一致）。
const (
	// VideoActionLike 最近点赞。
	VideoActionLike = "like"
	// VideoActionCoin 最近投币。
	VideoActionCoin = "coin"
)

// UserVideoQuery 用户主页视频列表与最近动作查询端口。
//
// 绑定到 video 上下文，返回其读模型，由接口层通过 res.OkWithList / res.OkWithData 序列化。JSON 字段遵守接口契约。
type UserVideoQuery interface {
	// ListUserVideos 用户主页视频分页列表。
	ListUserVideos(ctx context.Context, userID, viewerID uint, page, pageSize, sort, auditStatus int) (items any, total int64, err error)
	// RecentActions 最近点赞/投币的视频列表（action 取 VideoActionLike / VideoActionCoin）。
	RecentActions(ctx context.Context, userID uint, action string, limit int) (items any, count int, err error)
}

// UserVideoStats 作者已发布视频的曝光/互动汇总。
type UserVideoStats struct {
	TotalLikes int64
	TotalViews int64
}

// PublishedVideoStatsQuery 作者已发布视频统计端口，绑定到 video 上下文。
//
// 通过端口复用 Redis 统计缓存与 singleflight 能力。
type PublishedVideoStatsQuery interface {
	AggregateByAuthor(ctx context.Context, authorUsername string) UserVideoStats
}

// VideoTagWriter 标签创建端口。
//
// 跨上下文依赖：应绑定到 video 上下文。
// 返回值原样交给 res.OkWithData，实现须返回 video 上下文自己的标签读模型。
type VideoTagWriter interface {
	CreateOrGetTag(ctx context.Context, name string) (any, error)
}

// VideoRemover 用户删除自己视频的端口。
//
// 跨上下文依赖：应绑定到 video 上下文。
type VideoRemover interface {
	DeleteVideo(ctx context.Context, videoID, userID uint) error
}

// VideoAuthorRenamer 视频侧作者用户名级联更新端口。
//
// 绑定到 video 上下文，更新 videos.author_username、video_reports.reporter_username 与 danmu_reports.reporter_username。
// identity 在 tx.Manager 的事务 context 中调用；实现必须经 gormx.DB(ctx, root) 取库，使级联更新与用户改名处于同一事务。
type VideoAuthorRenamer interface {
	RenameAuthor(ctx context.Context, oldUsername, newUsername string) error
}

// VideoCacheInvalidator 视频侧缓存失效端口。
//
// 跨上下文依赖：应绑定到 video 上下文。
type VideoCacheInvalidator interface {
	// InvalidateUsername 失效「userID → username」派生缓存。
	InvalidateUsername(userID uint)
	// InvalidateAuthorVideos 批量失效该作者名下视频的详情缓存；
	// usernameChanged 为 true 时连同 authorID 缓存一起清。
	InvalidateAuthorVideos(ctx context.Context, authorUsername string, usernameChanged bool)
}
