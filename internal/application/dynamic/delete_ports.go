package dynamic

import (
	"context"
)

// DynamicCommentPurger 动态评论级联清理端口（由 comment 上下文实现）。
//
// 删除动态时需要同时删除其评论与评论点赞，评论聚合的写侧归 comment 上下文所有；
// 实现须复用 context 中的事务句柄，保证与动态删除处于同一事务。
type DynamicCommentPurger interface {
	// PurgeByDynamic 删除该动态下的全部评论与评论点赞，返回各自删除行数。
	PurgeByDynamic(ctx context.Context, dynamicID uint) (comments int64, commentLikes int64, err error)
}
