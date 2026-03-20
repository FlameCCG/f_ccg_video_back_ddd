package dynamic

import (
	"context"
)

// AdminDynamicQuery 管理端动态列表查询端口（读侧）。
type AdminDynamicQuery interface {
	// List 按用户/用户名/关键字分页查询动态，按「置顶优先 + 时间倒序」排序。
	List(ctx context.Context, params AdminDynamicListParams) ([]DynamicItem, int64, error)
}
