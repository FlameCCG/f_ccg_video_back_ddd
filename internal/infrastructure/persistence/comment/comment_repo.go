package comment

import (
	"context"
	"errors"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/comment"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// Repository 评论仓储的 GORM 实现。
type Repository struct {
	db *gorm.DB
}

var _ domain.Repository = (*Repository)(nil)

// NewRepository 构造评论仓储。
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// FindByID 读取完整评论；不存在时返回 (nil, nil)。
func (r *Repository) FindByID(ctx context.Context, commentID uint) (*domain.Comment, error) {
	return r.findOne(ctx, commentID)
}

// FindPinTarget 只读 id/video_id/dynamic_id/root_id，供置顶鉴权使用。
func (r *Repository) FindPinTarget(ctx context.Context, commentID uint) (*domain.Comment, error) {
	return r.findOne(ctx, commentID, "id", "video_id", "dynamic_id", "root_id")
}

// FindLikeTarget 只读 id/video_id/like_count，供点赞前置校验使用。
func (r *Repository) FindLikeTarget(ctx context.Context, commentID uint) (*domain.Comment, error) {
	return r.findOne(ctx, commentID, "id", "video_id", "like_count")
}

// FindNoticeSource 只读 id/user_id/video_id/dynamic_id/content，供点赞通知渲染使用。
func (r *Repository) FindNoticeSource(ctx context.Context, commentID uint) (*domain.Comment, error) {
	return r.findOne(ctx, commentID, "id", "user_id", "video_id", "dynamic_id", "content")
}

// findOne 按主键读取单条评论；columns 为空表示读取全部列。
func (r *Repository) findOne(ctx context.Context, commentID uint, columns ...string) (*domain.Comment, error) {
	query := gormx.DB(ctx, r.db)
	if len(columns) > 0 {
		query = query.Select(columns[0], toAnySlice(columns[1:])...)
	}

	var row po.Comment
	if err := query.Where("id = ?", commentID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomainComment(&row), nil
}

// BriefByIDs 批量只读 id/video_id/root_id。
func (r *Repository) BriefByIDs(ctx context.Context, commentIDs []uint) ([]domain.Comment, error) {
	if len(commentIDs) == 0 {
		return nil, nil
	}

	var rows []po.Comment
	if err := gormx.DB(ctx, r.db).
		Select("id", "video_id", "root_id").
		Find(&rows, "id IN ?", commentIDs).Error; err != nil {
		return nil, err
	}

	return toDomainComments(rows), nil
}

// ChildrenBrief 批量只读指定根评论下子评论的 id/video_id/root_id。
func (r *Repository) ChildrenBrief(ctx context.Context, rootIDs []uint) ([]domain.Comment, error) {
	if len(rootIDs) == 0 {
		return nil, nil
	}

	var rows []po.Comment
	if err := gormx.DB(ctx, r.db).
		Select("id", "video_id", "root_id").
		Find(&rows, "root_id IN ?", rootIDs).Error; err != nil {
		return nil, err
	}

	return toDomainComments(rows), nil
}

// ChildIDs 读取指定根评论下的全部子评论 ID。
func (r *Repository) ChildIDs(ctx context.Context, rootID uint) ([]uint, error) {
	var childIDs []uint
	if err := gormx.DB(ctx, r.db).Model(&po.Comment{}).
		Where("root_id = ?", rootID).
		Pluck("id", &childIDs).Error; err != nil {
		return nil, err
	}

	return childIDs, nil
}

// Create 写入评论并回填自增 ID 与创建时间。
func (r *Repository) Create(ctx context.Context, c *domain.Comment) error {
	if c == nil {
		return nil
	}

	row := toPOComment(c)
	if err := gormx.DB(ctx, r.db).Create(row).Error; err != nil {
		return err
	}

	c.ID = row.ID
	c.CreatedAt = row.CreatedAt
	c.UpdatedAt = row.UpdatedAt

	return nil
}

// IncrReplyCount 根评论回复数 +1。
func (r *Repository) IncrReplyCount(ctx context.Context, rootID uint) error {
	return gormx.DB(ctx, r.db).Model(&po.Comment{}).
		Where("id = ?", rootID).
		Update("reply_count", gorm.Expr("reply_count + 1")).Error
}

// DecrReplyCount 根评论回复数 -1（下界 0）。
func (r *Repository) DecrReplyCount(ctx context.Context, rootID uint) error {
	return gormx.DB(ctx, r.db).Model(&po.Comment{}).
		Where("id = ?", rootID).
		Update("reply_count", gorm.Expr("GREATEST(reply_count - 1, 0)")).Error
}

// DecrReplyCountBy 根评论回复数 -delta（下界 0）。
func (r *Repository) DecrReplyCountBy(ctx context.Context, rootID uint, delta int64) error {
	return gormx.DB(ctx, r.db).Model(&po.Comment{}).
		Where("id = ?", rootID).
		UpdateColumn("reply_count", gorm.Expr("GREATEST(reply_count - ?, 0)", delta)).Error
}

// DeleteByID 删除单条评论。
func (r *Repository) DeleteByID(ctx context.Context, commentID uint) error {
	return gormx.DB(ctx, r.db).Where("id = ?", commentID).Delete(&po.Comment{}).Error
}

// DeleteByIDs 批量删除评论。
func (r *Repository) DeleteByIDs(ctx context.Context, commentIDs []uint) error {
	if len(commentIDs) == 0 {
		return nil
	}

	return gormx.DB(ctx, r.db).Where("id IN ?", commentIDs).Delete(&po.Comment{}).Error
}

// DeleteByRootID 删除指定根评论下的全部子评论。
func (r *Repository) DeleteByRootID(ctx context.Context, rootID uint) error {
	return gormx.DB(ctx, r.db).Where("root_id = ?", rootID).Delete(&po.Comment{}).Error
}

// UnpinByVideo 取消指定视频下已置顶的一级评论。
func (r *Repository) UnpinByVideo(ctx context.Context, videoID uint) error {
	return gormx.DB(ctx, r.db).Model(&po.Comment{}).
		Where("video_id = ? AND root_id IS NULL AND is_pinned = ?", videoID, true).
		Updates(map[string]any{"is_pinned": false, "pinned_at": nil}).Error
}

// UnpinByDynamic 取消指定动态下已置顶的一级评论。
func (r *Repository) UnpinByDynamic(ctx context.Context, dynamicID uint) error {
	return gormx.DB(ctx, r.db).Model(&po.Comment{}).
		Where("dynamic_id = ? AND root_id IS NULL AND is_pinned = ?", dynamicID, true).
		Updates(map[string]any{"is_pinned": false, "pinned_at": nil}).Error
}

// SetPinned 设置单条评论的置顶状态与置顶时间。
func (r *Repository) SetPinned(ctx context.Context, commentID uint, pinned bool, pinnedAt *time.Time) error {
	return gormx.DB(ctx, r.db).Model(&po.Comment{}).
		Where("id = ?", commentID).
		Updates(map[string]any{"is_pinned": pinned, "pinned_at": pinnedAt}).Error
}

// IncrLikeCount 评论点赞数 +1。
func (r *Repository) IncrLikeCount(ctx context.Context, commentID uint) error {
	return gormx.DB(ctx, r.db).Model(&po.Comment{}).
		Where("id = ?", commentID).
		Update("like_count", gorm.Expr("like_count + 1")).Error
}

// DecrLikeCount 评论点赞数 -1（下界 0）。
func (r *Repository) DecrLikeCount(ctx context.Context, commentID uint) error {
	return gormx.DB(ctx, r.db).Model(&po.Comment{}).
		Where("id = ?", commentID).
		Update("like_count", gorm.Expr("GREATEST(like_count - 1, 0)")).Error
}

// LikeCountByID 读取评论点赞数；评论不存在时返回 (0, nil)。
func (r *Repository) LikeCountByID(ctx context.Context, commentID uint) (int64, error) {
	var row po.Comment
	if err := gormx.DB(ctx, r.db).
		Select("like_count").
		Where("id = ?", commentID).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}

		return 0, err
	}

	return row.LikeCount, nil
}

// LikeRepository 评论点赞仓储的 GORM 实现。
type LikeRepository struct {
	db *gorm.DB
}

var _ domain.LikeRepository = (*LikeRepository)(nil)

// NewLikeRepository 构造评论点赞仓储。
func NewLikeRepository(db *gorm.DB) *LikeRepository {
	return &LikeRepository{db: db}
}

// Find 查询单条点赞记录；不存在时返回 (nil, nil)。
func (r *LikeRepository) Find(ctx context.Context, commentID, userID uint) (*domain.Like, error) {
	var row po.CommentLike
	if err := gormx.DB(ctx, r.db).
		Where("comment_id = ? AND user_id = ?", commentID, userID).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomainLike(&row), nil
}

// Create 写入点赞记录并回填自增 ID。
func (r *LikeRepository) Create(ctx context.Context, like *domain.Like) error {
	if like == nil {
		return nil
	}

	row := toPOLike(like)
	if err := gormx.DB(ctx, r.db).Create(row).Error; err != nil {
		return err
	}

	like.ID = row.ID
	like.CreatedAt = row.CreatedAt
	like.UpdatedAt = row.UpdatedAt

	return nil
}

// DeleteByID 按主键删除点赞记录。
func (r *LikeRepository) DeleteByID(ctx context.Context, likeID uint) error {
	return gormx.DB(ctx, r.db).Where("id = ?", likeID).Delete(&po.CommentLike{}).Error
}

// DeleteByCommentIDs 删除指定评论的全部点赞记录。
func (r *LikeRepository) DeleteByCommentIDs(ctx context.Context, commentIDs []uint) error {
	if len(commentIDs) == 0 {
		return nil
	}

	return gormx.DB(ctx, r.db).Where("comment_id IN ?", commentIDs).Delete(&po.CommentLike{}).Error
}

// UserIDsByComment 读取某条评论的全部点赞用户 ID。
func (r *LikeRepository) UserIDsByComment(ctx context.Context, commentID uint) ([]uint, error) {
	var userIDs []uint
	if err := gormx.DB(ctx, r.db).Model(&po.CommentLike{}).
		Where("comment_id = ?", commentID).
		Pluck("user_id", &userIDs).Error; err != nil {
		return nil, err
	}

	return userIDs, nil
}

// LikedCommentIDs 批量判断用户是否点赞过指定评论（仅返回已点赞项）。
func (r *LikeRepository) LikedCommentIDs(
	ctx context.Context,
	userID uint,
	commentIDs []uint,
) (map[uint]bool, error) {
	likedMap := make(map[uint]bool)
	if userID == 0 || len(commentIDs) == 0 {
		return likedMap, nil
	}

	var likes []struct {
		CommentID uint `gorm:"column:comment_id"`
	}
	if err := gormx.DB(ctx, r.db).
		Model(&po.CommentLike{}).
		Select("comment_id").
		Where("user_id = ? AND comment_id IN ?", userID, commentIDs).
		Find(&likes).Error; err != nil {
		return likedMap, err
	}

	for _, l := range likes {
		likedMap[l.CommentID] = true
	}

	return likedMap, nil
}

// toAnySlice 把列名切片转为 GORM Select 的可变参数形式。
func toAnySlice(columns []string) []any {
	if len(columns) == 0 {
		return nil
	}

	args := make([]any, 0, len(columns))
	for _, c := range columns {
		args = append(args, c)
	}

	return args
}
