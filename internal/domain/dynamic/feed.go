package dynamic

import "f_ccg_video_back_ddd/internal/domain/shared/enum"

// 作品流分页边界（与原 dynamic_service 常量一致）。
const (
	// DefaultFeedPageSize 作品流默认每页条数。
	DefaultFeedPageSize = 20
	// MaxFeedPageSize 作品流每页条数上限。
	MaxFeedPageSize = 50
	// DefaultAdminPageSize 管理端动态列表默认每页条数。
	DefaultAdminPageSize = 20
	// MaxAdminPageSize 管理端动态列表每页条数上限。
	MaxAdminPageSize = 100
)

// FeedSources 描述一次作品流查询应包含的作品来源。
type FeedSources struct {
	IncludeVideo   bool
	IncludeDynamic bool
}

// Any 是否至少包含一种作品来源。
func (s FeedSources) Any() bool {
	return s.IncludeVideo || s.IncludeDynamic
}

// BuildFeedSources 按请求的作品类型解析应查询的来源。
//
// 规则与原 listAuthorFeed 一致：0 表示视频与动态全都要；
// 1 只要视频；2 只要动态；其它取值不匹配任何来源（调用方应直接返回空列表）。
func BuildFeedSources(workType int) FeedSources {
	return FeedSources{
		IncludeVideo:   workType == 0 || workType == int(enum.WorkTypeVideo),
		IncludeDynamic: workType == 0 || workType == int(enum.WorkTypeDynamic),
	}
}

// NormalizeFeedPaging 纠偏作品流分页参数（与原 ListFeed / ListFollowUsers 一致）：
// 页码下限 1；每页条数非正时取默认值，超过上限时截断到上限。
func NormalizeFeedPaging(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 {
		pageSize = DefaultFeedPageSize
	}

	if pageSize > MaxFeedPageSize {
		pageSize = MaxFeedPageSize
	}

	return page, pageSize
}

// NormalizeAdminPaging 纠偏管理端动态列表分页参数（与原 AdminList 一致）：
// 页码下限 1；每页条数非正或超过上限时统一回落默认值。
func NormalizeAdminPaging(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 || pageSize > MaxAdminPageSize {
		pageSize = DefaultAdminPageSize
	}

	return page, pageSize
}
