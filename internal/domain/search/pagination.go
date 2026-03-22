package search

const (
	// defaultPageSize 搜索默认分页大小.
	defaultPageSize = 20
	// maxPageSize 搜索分页上限，防止深分页.
	maxPageSize = 100

	// defaultSuggestSize 联想默认候选数.
	defaultSuggestSize = 10
	// maxSuggestSize 联想候选数上限.
	maxSuggestSize = 20

	// defaultMentionPageSize @ 提及默认分页大小.
	defaultMentionPageSize = 20
	// maxMentionPageSize @ 提及分页上限.
	maxMentionPageSize = 50

	// defaultHotKeywordPageSize 热搜榜默认分页大小.
	defaultHotKeywordPageSize = 10
	// maxHotKeywordPageSize 热搜榜分页上限.
	maxHotKeywordPageSize = 20

	// fallbackHotKeywordSampleMinSize 热词回退采样的最小行数.
	fallbackHotKeywordSampleMinSize = 50
	// fallbackHotKeywordSampleMultiple 热词回退采样的放大倍数（去重后仍够一页）.
	fallbackHotKeywordSampleMultiple = 5
)

// Pagination 是规范化后的分页参数（页码从 1 开始）.
type Pagination struct {
	Page     int
	PageSize int
}

// Offset 返回 SQL/ES 使用的偏移量，最小为 0.
func (p Pagination) Offset() int {
	return max((p.Page-1)*p.PageSize, 0)
}

// PaginationPolicy 是搜索、联想、提及和热词的分页与排序上限。
//
// 零值即可使用。它不负责回退查询或结果拼装。
type PaginationPolicy struct{}

// NormalizePagination 规范化搜索分页：页码下限 1，页大小缺省 20、上限 100.
func (p PaginationPolicy) NormalizePagination(page, pageSize int) Pagination {
	return p.normalize(page, pageSize, defaultPageSize, maxPageSize)
}

// NormalizeMentionPagination 规范化 @ 提及分页：页码下限 1，页大小缺省 20、上限 50.
func (p PaginationPolicy) NormalizeMentionPagination(page, pageSize int) Pagination {
	return p.normalize(page, pageSize, defaultMentionPageSize, maxMentionPageSize)
}

// NormalizeHotKeywordPagination 规范化热搜榜分页：页码下限 1，页大小缺省 10、上限 20.
func (p PaginationPolicy) NormalizeHotKeywordPagination(page, pageSize int) Pagination {
	return p.normalize(page, pageSize, defaultHotKeywordPageSize, maxHotKeywordPageSize)
}

func (PaginationPolicy) normalize(page, pageSize, defaultSize, maxSize int) Pagination {
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 {
		pageSize = defaultSize
	}

	if pageSize > maxSize {
		pageSize = maxSize
	}

	return Pagination{Page: page, PageSize: pageSize}
}

// NormalizeSuggestSize 规范化联想候选数：缺省 10、上限 20.
func (PaginationPolicy) NormalizeSuggestSize(size int) int {
	if size <= 0 {
		return defaultSuggestSize
	}

	if size > maxSuggestSize {
		return maxSuggestSize
	}

	return size
}

// HotKeywordSampleSize 计算热词回退采样行数：按目标页放大 5 倍，且不低于 50 行。
// 采样后要按标题去重，样本必须显著大于页大小才能填满一页。
func (PaginationPolicy) HotKeywordSampleSize(page, pageSize int) int {
	return max(page*pageSize*fallbackHotKeywordSampleMultiple, fallbackHotKeywordSampleMinSize)
}

// SortSpec 是规范化后的排序描述.
type SortSpec struct {
	// Field 排序字段枚举值.
	Field int
	// Order 排序方向：SortOrderDesc / SortOrderAsc.
	Order int
}

// NormalizeVideoSort 规范化视频排序：字段越界回落播放量，方向仅显式 1 才升序。
func (p PaginationPolicy) NormalizeVideoSort(field, order int) SortSpec {
	if field < VideoSortRelevance || field > VideoSortDuration {
		field = VideoSortViews
	}

	return SortSpec{Field: field, Order: p.normalizeSortOrder(order)}
}

// NormalizeUserSort 规范化用户排序：字段越界回落粉丝数，方向仅显式 1 才升序。
func (p PaginationPolicy) NormalizeUserSort(field, order int) SortSpec {
	if field < UserSortRelevance || field > UserSortCreatedAt {
		field = UserSortFollowers
	}

	return SortSpec{Field: field, Order: p.normalizeSortOrder(order)}
}

func (PaginationPolicy) normalizeSortOrder(order int) int {
	if order != SortOrderAsc {
		return SortOrderDesc
	}

	return SortOrderAsc
}

// SliceWindow 返回切片 [start,end) 的分页窗口下标；越界时返回空窗口（start==end）。
func (PaginationPolicy) SliceWindow(total int, page Pagination) (int, int) {
	if total <= 0 {
		return 0, 0
	}

	start := page.Offset()
	if start >= total {
		return 0, 0
	}

	return start, min(start+page.PageSize, total)
}
