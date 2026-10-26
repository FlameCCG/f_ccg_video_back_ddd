package scheduler

import (
	"context"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	app "f_ccg_video_back_ddd/internal/application/platform"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CommentLikeProjection 评论点赞写侧投影的 GORM 实现（直连 comments / comment_likes 表）。
type CommentLikeProjection struct {
	db *gorm.DB
}

var _ app.CommentLikeProjection = (*CommentLikeProjection)(nil)

// NewCommentLikeProjection 构造评论点赞投影仓储。
func NewCommentLikeProjection(db *gorm.DB) *CommentLikeProjection {
	return &CommentLikeProjection{db: db}
}

// CommentExists 报告评论是否仍存在（LIMIT 1 计数，语义与原实现一致）。
func (p *CommentLikeProjection) CommentExists(ctx context.Context, commentID uint) (bool, error) {
	var count int64
	if err := gormx.DB(ctx, p.db).
		Model(&po.Comment{}).
		Where("id = ?", commentID).
		Limit(1).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

// InsertLike 插入点赞行；唯一键冲突时不报错且返回 false（已赞，计数不动）。
func (p *CommentLikeProjection) InsertLike(ctx context.Context, commentID, userID uint) (bool, error) {
	res := gormx.DB(ctx, p.db).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&po.CommentLike{
			CommentID: commentID,
			UserID:    userID,
		})
	if res.Error != nil {
		return false, res.Error
	}

	return res.RowsAffected > 0, nil
}

// DeleteLike 删除点赞行；本无该行时返回 false（最终未赞，计数不动）。
func (p *CommentLikeProjection) DeleteLike(ctx context.Context, commentID, userID uint) (bool, error) {
	res := gormx.DB(ctx, p.db).
		Where("comment_id = ? AND user_id = ?", commentID, userID).
		Delete(&po.CommentLike{})
	if res.Error != nil {
		return false, res.Error
	}

	return res.RowsAffected > 0, nil
}

// IncrLikeCount 评论点赞数 +1。
func (p *CommentLikeProjection) IncrLikeCount(ctx context.Context, commentID uint) error {
	return gormx.DB(ctx, p.db).
		Model(&po.Comment{}).
		Where("id = ?", commentID).
		Update("like_count", gorm.Expr("like_count + 1")).Error
}

// DecrLikeCount 评论点赞数 -1，下限为 0（SQL 表达式与原实现逐字一致）。
func (p *CommentLikeProjection) DecrLikeCount(ctx context.Context, commentID uint) error {
	return gormx.DB(ctx, p.db).
		Model(&po.Comment{}).
		Where("id = ?", commentID).
		Update("like_count", gorm.Expr("GREATEST(like_count - 1, 0)")).Error
}
