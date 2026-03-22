package search

import (
	"context"

	searchdomain "f_ccg_video_back_ddd/internal/domain/search"
)

// applyNormalized 把规范化后的分页与排序写回搜索参数。
func (s *SearchService) applyNormalized(opts SearchOptions, page searchdomain.Pagination, videoSort, userSort searchdomain.SortSpec) SearchOptions {
	opts.Page, opts.PageSize = page.Page, page.PageSize
	opts.VideoSort, opts.VideoOrder = videoSort.Field, videoSort.Order
	opts.UserSort, opts.UserOrder = userSort.Field, userSort.Order

	return opts
}

// callContext 保证向下游传递非 nil 上下文。
type callContext struct{}

func (callContext) ensure(ctx context.Context) context.Context {
	if ctx == nil {
		return context.TODO()
	}

	return ctx
}
