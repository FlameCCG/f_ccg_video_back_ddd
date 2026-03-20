package dynamic

import (
	"context"
)

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

// UnreadFollowCountCache 关注未读数缓存端口（Redis stale-while-revalidate）。
//
// Redis key 前缀与 TTL 属于外部契约，由 infrastructure/cache/dynamic 实现。
type UnreadFollowCountCache interface {
	// Load 读取缓存；返回值分别为计数、是否新鲜、是否命中。
	Load(userID uint) (int64, bool, bool)
	// Store 写入缓存（以 stale TTL 作为 Redis 过期时间）。
	Store(userID uint, count int64)
	// Invalidate 删除指定用户的未读数缓存。
	Invalidate(userID uint)
}
