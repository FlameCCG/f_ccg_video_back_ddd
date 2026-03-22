package video

import "strings"

// ListingPolicy 归一化分页、关键字与最近互动资格，各列表分别使用对应的默认值和上限。
type ListingPolicy struct{}

// RankWindow 是 Top100 榜单的有效窗口，Limit 为零表示当前页没有结果。
type RankWindow struct {
	Total  int64
	Offset int64
	Limit  int
}

// RankWindow 统一 Redis 与数据库回退的榜单上限和末页裁剪。
func (p ListingPolicy) RankWindow(cardinality int64, page, pageSize int) RankWindow {
	page, pageSize = p.RankPage(page, pageSize)
	window := RankWindow{Total: min(cardinality, 100)}
	if window.Total <= 0 || int64(page-1) > (window.Total-1)/int64(pageSize) {
		return window
	}
	window.Offset = int64(page-1) * int64(pageSize)
	window.Limit = int(min(int64(pageSize), window.Total-window.Offset))
	return window
}

// UserVideoPage 规范用户主页视频列表的分页参数。
//
// 页码下限 1，页大小超出 (0, 50] 时回落 20。历史列表与创作中心弹幕列表使用同一规则。
func (ListingPolicy) UserVideoPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}

	const maxUserVideoPageSize = 50
	if pageSize <= 0 || pageSize > maxUserVideoPageSize {
		pageSize = 20
	}

	return page, pageSize
}

// RecentLimit 规范「最近点赞/投币」的取数上限。超出 (0, 50] 时回落 10。
func (ListingPolicy) RecentLimit(limit int) int {
	const maxRecentActionLimit = 50
	if limit <= 0 || limit > maxRecentActionLimit {
		return 10
	}

	return limit
}

// RecentAction 校验最近互动动作类型。
func (ListingPolicy) RecentAction(action string) error {
	switch action {
	case ActionLike, ActionCoin:
		return nil
	default:
		return ErrActionUnsupported
	}
}

// Keyword 裁剪搜索关键字两端空白。
func (ListingPolicy) Keyword(keyword string) string {
	return strings.TrimSpace(keyword)
}

// FolderVideoPage 规范收藏夹内视频分页：页码下限 1，空页大小回落 20，超过 50 截断为 50。
func (ListingPolicy) FolderVideoPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 {
		pageSize = 20
	}

	if pageSize > 50 {
		pageSize = 50
	}

	return page, pageSize
}

// AdminPage 规范管理端列表分页：页码下限 1，页大小超出 (0, 100] 时回落 20。
func (ListingPolicy) AdminPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	return page, pageSize
}

// FeedPage 规范首页流分页。默认 11，上限 50。
func (ListingPolicy) FeedPage(page, pageSize int) (int, int) {
	return (ListingPolicy{}).bounded(page, pageSize, 11, 50)
}

// HotPage 规范综合热门分页。默认 20，上限 50。
func (ListingPolicy) HotPage(page, pageSize int) (int, int) {
	return (ListingPolicy{}).bounded(page, pageSize, 20, 50)
}

// RankPage 规范排行榜分页。默认 20，上限 50。
func (ListingPolicy) RankPage(page, pageSize int) (int, int) {
	return (ListingPolicy{}).bounded(page, pageSize, 20, 50)
}

// RelatedSize 规范详情页相关推荐条数。默认 12，上限 50。
func (ListingPolicy) RelatedSize(size int) int {
	if size <= 0 {
		return 12
	}

	if size > 50 {
		return 50
	}

	return size
}

func (ListingPolicy) bounded(page, pageSize, defaultSize, maxSize int) (int, int) {
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 {
		pageSize = defaultSize
	}

	if pageSize > maxSize {
		pageSize = maxSize
	}

	return page, pageSize
}
