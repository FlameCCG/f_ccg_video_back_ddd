package comment

import (
	"context"
)

type CommentLikeProjection interface {
	// CommentExists 报告评论是否仍存在。
	CommentExists(ctx context.Context, commentID uint) (bool, error)
	// InsertLike 插入点赞行；已存在（唯一键冲突）时返回 false。
	InsertLike(ctx context.Context, commentID, userID uint) (bool, error)
	// DeleteLike 删除点赞行；本无该行时返回 false。
	DeleteLike(ctx context.Context, commentID, userID uint) (bool, error)
	// IncrLikeCount 评论点赞数 +1。
	IncrLikeCount(ctx context.Context, commentID uint) error
	// DecrLikeCount 评论点赞数 -1，下限为 0。
	DecrLikeCount(ctx context.Context, commentID uint) error
}
