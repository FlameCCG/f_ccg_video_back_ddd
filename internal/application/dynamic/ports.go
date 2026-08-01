package dynamic

import (
	"context"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/dynamic"
)

// ---------------------------------------------------------------------------
// 读侧查询端口：实现全部落在本上下文的 infrastructure/persistence/dynamic。
// 其中跨表读取（users / videos / comments / video_likes / user_focus）按
// docs/decisions.md D-011「读侧投影允许直读其它上下文的表」执行，
// 写侧一律走本文件后半部分的协作端口。
// ---------------------------------------------------------------------------

// AdminDynamicQuery 管理端动态列表查询端口（读侧）。
type AdminDynamicQuery interface {
	// List 按用户/用户名/关键字分页查询动态，按「置顶优先 + 时间倒序」排序。
	List(ctx context.Context, params AdminDynamicListParams) ([]DynamicItem, int64, error)
}

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

// ActivityQuery 作品活跃度与已读位点查询端口（读侧）。
type ActivityQuery interface {
	// LatestVideoTimes 批量读取作者最新已发布视频时间（key 为作者用户名）。
	LatestVideoTimes(ctx context.Context, authorUsernames []string) (map[string]time.Time, error)
	// LatestDynamicTimes 批量读取作者最新动态时间（key 为作者用户 ID）。
	LatestDynamicTimes(ctx context.Context, authorIDs []uint) (map[uint]time.Time, error)
	// LastReadTimes 批量读取当前用户对各作者的已读位点时间。
	LastReadTimes(ctx context.Context, userID uint, authorIDs []uint) (map[uint]time.Time, error)
	// LatestVideoWork 读取作者最新一条已发布视频；不存在时返回零值 WorkRef。
	LatestVideoWork(ctx context.Context, authorUsername string) (domain.WorkRef, error)
	// LatestDynamicWork 读取作者最新一条动态；不存在时返回零值 WorkRef。
	LatestDynamicWork(ctx context.Context, authorID uint) (domain.WorkRef, error)
	// PublishedVideoAuthor 读取已发布视频及其作者引用；视频不存在或未发布时返回零值。
	PublishedVideoAuthor(ctx context.Context, videoID uint) (VideoAuthorRef, error)
}

// FollowRelationQuery 关注关系读侧端口。
//
// 关注关系的聚合归 social 上下文所有，dynamic 只做读侧投影，
// 因此在本包声明本地端口，绝不 import social 上下文。
type FollowRelationQuery interface {
	// IsFollowing 判定 userID 是否关注了 authorID；查询失败按「未关注」处理。
	IsFollowing(ctx context.Context, userID, authorID uint) bool
	// ListFollowing 列出 userID 关注的全部用户（含用户名与头像）。
	ListFollowing(ctx context.Context, userID uint) ([]FollowRelation, error)
}

// UserProfileQuery 用户资料读侧端口（id / username / avatar）。
type UserProfileQuery interface {
	// ProfileByID 读取用户资料摘要；用户不存在时返回 (nil, nil)。
	ProfileByID(ctx context.Context, userID uint) (*UserProfile, error)
	// UsernameByID 读取用户名；用户不存在时返回空串。
	UsernameByID(ctx context.Context, userID uint) (string, error)
}

// VideoOwnershipQuery 视频归属读侧端口。
type VideoOwnershipQuery interface {
	// ExistsPublishedByAuthor 判定视频是否存在、已发布且属于指定作者。
	ExistsPublishedByAuthor(ctx context.Context, videoID uint, authorUsername string) (bool, error)
}

// UnreadFollowCountCache 关注未读数缓存端口（Redis stale-while-revalidate）。
//
// Redis key 前缀与 TTL 属于外部契约，由 infrastructure/cache/dynamic 保持与原实现一致。
type UnreadFollowCountCache interface {
	// Load 读取缓存；返回值分别为计数、是否新鲜、是否命中。
	Load(userID uint) (int64, bool, bool)
	// Store 写入缓存（以 stale TTL 作为 Redis 过期时间）。
	Store(userID uint, count int64)
	// Invalidate 删除指定用户的未读数缓存。
	Invalidate(userID uint)
}

// ---------------------------------------------------------------------------
// 跨上下文写侧协作端口：由 bootstrap 用 wire.Bind 绑定到对应上下文的实现。
// ---------------------------------------------------------------------------

// VideoPinWriter 视频置顶写侧端口（由 video 上下文实现）。
//
// 动态与视频的置顶互斥规则要求本上下文能取消作者已置顶的视频，
// 但视频聚合的写侧归 video 上下文所有，故以端口协作。
type VideoPinWriter interface {
	// ClearPinnedByAuthor 取消该作者当前已置顶的全部视频。
	ClearPinnedByAuthor(ctx context.Context, authorUsername string) error
	// SetPinned 把作者名下指定视频标记为置顶，并写入置顶时间。
	SetPinned(ctx context.Context, videoID uint, authorUsername string, pinnedAt *time.Time) error
	// ClearPinned 取消作者名下指定视频的置顶状态。
	ClearPinned(ctx context.Context, videoID uint, authorUsername string) error
}

// DynamicCommentPurger 动态评论级联清理端口（由 comment 上下文实现）。
//
// 删除动态时需要同时删除其评论与评论点赞，评论聚合的写侧归 comment 上下文所有；
// 实现须复用 context 中的事务句柄，保证与动态删除处于同一事务。
type DynamicCommentPurger interface {
	// PurgeByDynamic 删除该动态下的全部评论与评论点赞，返回各自删除行数。
	PurgeByDynamic(ctx context.Context, dynamicID uint) (comments int64, commentLikes int64, err error)
}

// DynamicLikeNotifier 动态点赞通知端口（由 notification 上下文实现）。
type DynamicLikeNotifier interface {
	// NotifyDynamicLike 向动态作者发送「有人点赞了你的动态」通知。
	NotifyDynamicLike(
		ctx context.Context,
		receiverID uint,
		dynamicID uint,
		actionUserID uint,
		actionUserName, actionUserAvatar, title, content string,
	) error
}
