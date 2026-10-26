package dynamic

import (
	"context"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// Repository 动态聚合的写侧仓储（接口所有权归领域层）。
type Repository interface {
	// FindByID 按主键读取动态；不存在时返回 (nil, nil)。
	FindByID(ctx context.Context, dynamicID uint) (*Dynamic, error)
	// ExistsByID 探测动态是否存在（原实现以 select id/user_id/like_count 的 Take 判定）。
	ExistsByID(ctx context.Context, dynamicID uint) (bool, error)
	// FindNoticeTargetByID 只读 id/user_id/content，供点赞通知渲染；不存在时返回 (nil, nil)。
	FindNoticeTargetByID(ctx context.Context, dynamicID uint) (*Dynamic, error)
	// Create 写入动态，并回填自增 ID 与时间戳。
	Create(ctx context.Context, d *Dynamic) error
	// DeleteByID 删除单条动态。
	DeleteByID(ctx context.Context, dynamicID uint) error
	// DeleteByIDs 批量删除动态，返回受影响行数。
	DeleteByIDs(ctx context.Context, dynamicIDs []uint) (int64, error)
	// ClearPinnedByUser 清除该用户当前已置顶的动态。
	ClearPinnedByUser(ctx context.Context, userID uint) error
	// SetPinned 把指定动态标记为置顶，并写入置顶时间。
	SetPinned(ctx context.Context, dynamicID uint, pinnedAt *time.Time) error
	// ClearPinned 取消指定动态的置顶状态。
	ClearPinned(ctx context.Context, dynamicID uint) error
	// IncrLikeCount 动态点赞数 +1。
	IncrLikeCount(ctx context.Context, dynamicID uint) error
	// DecrLikeCount 动态点赞数 -1（不低于 0）。
	DecrLikeCount(ctx context.Context, dynamicID uint) error
	// LikeCount 读取动态当前点赞数。
	LikeCount(ctx context.Context, dynamicID uint) (int64, error)
}

// LikeRepository 动态点赞记录仓储。
type LikeRepository interface {
	// Create 写入点赞记录；唯一键冲突时返回 ErrLikeDuplicated。
	Create(ctx context.Context, dynamicID, userID uint) error
	// Delete 删除点赞记录，返回受影响行数。
	Delete(ctx context.Context, dynamicID, userID uint) (int64, error)
}

// ReadRepository 「最新作品已读位点」仓储。
type ReadRepository interface {
	// FindByUserAndAuthor 读取用户对某作者的已读位点；不存在时返回 (nil, nil)。
	FindByUserAndAuthor(ctx context.Context, userID, authorID uint) (*ReadRecord, error)
	// Create 写入一条新的已读位点。
	Create(ctx context.Context, record *ReadRecord) error
	// Advance 把已存在的已读位点推进到指定作品。
	Advance(ctx context.Context, id uint, workType enum.WorkType, workID uint, workAt time.Time) error
}
