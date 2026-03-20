package search

import (
	"context"

	searchvo "f_ccg_video_back_ddd/internal/domain/search/valueobject"
)

// applyNormalized 把规范化后的分页与排序写回搜索参数。
func applyNormalized(opts SearchOptions, page searchvo.Pagination, videoSort, userSort searchvo.SortSpec) SearchOptions {
	opts.Page, opts.PageSize = page.Page, page.PageSize
	opts.VideoSort, opts.VideoOrder = videoSort.Field, videoSort.Order
	opts.UserSort, opts.UserOrder = userSort.Field, userSort.Order

	return opts
}

// ensureContext 保证向下游传递非 nil 上下文。
func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.TODO()
	}

	return ctx
}
