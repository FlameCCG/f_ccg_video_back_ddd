package comment

import (
	"context"
	"time"
)

// Repository 评论聚合的写侧仓储（接口所有权归领域层）。
//
// 「不存在」一律用 (nil, nil) 表达，除非注释另有说明。
type Repository interface {
	// FindByID 读取完整评论；不存在时返回 (nil, nil)。
	FindByID(ctx context.Context, commentID uint) (*Comment, error)
	// FindPinTarget 只读 id/video_id/dynamic_id/root_id，供置顶鉴权使用；不存在时返回 (nil, nil)。
	FindPinTarget(ctx context.Context, commentID uint) (*Comment, error)
	// FindLikeTarget 只读 id/video_id/like_count，供点赞前置校验使用；不存在时返回 (nil, nil)。
	FindLikeTarget(ctx context.Context, commentID uint) (*Comment, error)
	// FindNoticeSource 只读 id/user_id/video_id/dynamic_id/content，供点赞通知渲染使用；
	// 不存在时返回 (nil, nil)。
	FindNoticeSource(ctx context.Context, commentID uint) (*Comment, error)
	// BriefByIDs 批量只读 id/video_id/root_id。
	BriefByIDs(ctx context.Context, commentIDs []uint) ([]Comment, error)
	// ChildrenBrief 批量只读指定根评论下子评论的 id/video_id/root_id。
	ChildrenBrief(ctx context.Context, rootIDs []uint) ([]Comment, error)
	// ChildIDs 读取指定根评论下的全部子评论 ID。
	ChildIDs(ctx context.Context, rootID uint) ([]uint, error)

	// Create 写入评论并回填自增 ID 与创建时间。
	Create(ctx context.Context, c *Comment) error
	// IncrReplyCount 根评论回复数 +1。
	IncrReplyCount(ctx context.Context, rootID uint) error
	// DecrReplyCount 根评论回复数 -1（下界 0）。
	DecrReplyCount(ctx context.Context, rootID uint) error
	// DecrReplyCountBy 根评论回复数 -delta（下界 0），管理端批量删除使用。
	DecrReplyCountBy(ctx context.Context, rootID uint, delta int64) error

	// DeleteByID 删除单条评论。
	DeleteByID(ctx context.Context, commentID uint) error
	// DeleteByIDs 批量删除评论。
	DeleteByIDs(ctx context.Context, commentIDs []uint) error
	// DeleteByRootID 删除指定根评论下的全部子评论。
	DeleteByRootID(ctx context.Context, rootID uint) error

	// UnpinByVideo 取消指定视频下已置顶的一级评论。
	UnpinByVideo(ctx context.Context, videoID uint) error
	// UnpinByDynamic 取消指定动态下已置顶的一级评论。
	UnpinByDynamic(ctx context.Context, dynamicID uint) error
	// SetPinned 设置单条评论的置顶状态与置顶时间。
	SetPinned(ctx context.Context, commentID uint, pinned bool, pinnedAt *time.Time) error

	// IncrLikeCount 评论点赞数 +1。
	IncrLikeCount(ctx context.Context, commentID uint) error
	// DecrLikeCount 评论点赞数 -1（下界 0）。
	DecrLikeCount(ctx context.Context, commentID uint) error
	// LikeCountByID 读取评论点赞数；不存在时返回 0。
	LikeCountByID(ctx context.Context, commentID uint) (int64, error)
}

// LikeRepository 评论点赞记录仓储。
type LikeRepository interface {
	// Find 查询单条点赞记录；不存在时返回 (nil, nil)。
	Find(ctx context.Context, commentID, userID uint) (*Like, error)
	// Create 写入点赞记录并回填自增 ID。
	Create(ctx context.Context, like *Like) error
	// DeleteByID 按主键删除点赞记录。
	DeleteByID(ctx context.Context, likeID uint) error
	// DeleteByCommentIDs 删除指定评论的全部点赞记录。
	DeleteByCommentIDs(ctx context.Context, commentIDs []uint) error
	// UserIDsByComment 读取某条评论的全部点赞用户 ID（缓存重建用）。
	UserIDsByComment(ctx context.Context, commentID uint) ([]uint, error)
	// LikedCommentIDs 批量判断用户是否点赞过指定评论（仅返回已点赞项）。
	LikedCommentIDs(ctx context.Context, userID uint, commentIDs []uint) (map[uint]bool, error)
}
