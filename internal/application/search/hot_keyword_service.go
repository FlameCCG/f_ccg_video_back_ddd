package search

import (
	"context"
	"math"

	domain "f_ccg_video_back_ddd/internal/domain/search"
)

// HotKeywordService 首页热搜词用例服务（原 service/search_service/hot_keywords.go）。
//
// 优先读取 Redis 热搜词榜；榜单为空或 Redis 不可用时回退到热门视频标题。
type HotKeywordService struct {
	store    HotKeywordStore
	fallback HotKeywordFallbackQuery
}

// NewHotKeywordService 构造热搜词服务。
func NewHotKeywordService(store HotKeywordStore, fallback HotKeywordFallbackQuery) *HotKeywordService {
	return &HotKeywordService{store: store, fallback: fallback}
}

// HotKeywords 返回热搜榜的一页数据与总量。
func (s *HotKeywordService) HotKeywords(ctx context.Context, page, pageSize int) ([]HotKeywordItem, int64, error) {
	ctx = ensureContext(ctx)
	pagination := domain.NormalizeHotKeywordPagination(page, pageSize)

	if !s.store.Available() {
		return s.fallbackHotKeywords(ctx, pagination)
	}

	total, err := s.store.Card(ctx)
	if err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return s.fallbackHotKeywords(ctx, pagination)
	}

	start := int64((pagination.Page - 1) * pagination.PageSize)

	end := start + int64(pagination.PageSize) - 1
	if start < 0 {
		start = 0
	}

	if end < start {
		end = start
	}

	members, err := s.store.RevRangeWithScores(ctx, start, end)
	if err != nil {
		return nil, 0, err
	}

	if len(members) == 0 {
		return []HotKeywordItem{}, total, nil
	}

	items := make([]HotKeywordItem, 0, len(members))

	for _, member := range members {
		keyword := domain.NormalizeHotKeyword(member.Member)
		if keyword == "" {
			continue
		}

		items = append(items, HotKeywordItem{
			Keyword: keyword,
			Score:   int64(math.Round(member.Score)),
		})
	}

	return items, total, nil
}

// RecordHotKeyword 记录搜索关键词热度，仅在 Redis 可用时生效。
func (s *HotKeywordService) RecordHotKeyword(ctx context.Context, keyword string) {
	keyword = domain.NormalizeHotKeyword(keyword)
	if keyword == "" || !s.store.Available() {
		return
	}

	s.store.Incr(ensureContext(ctx), keyword)
}

// fallbackHotKeywords 用热门视频标题拼装热搜榜（内存分页）。
func (s *HotKeywordService) fallbackHotKeywords(
	ctx context.Context,
	pagination domain.Pagination,
) ([]HotKeywordItem, int64, error) {
	if !s.fallback.Available() {
		return []HotKeywordItem{}, 0, nil
	}

	rows, err := s.fallback.HotVideoTitles(ctx, domain.HotKeywordSampleSize(pagination.Page, pagination.PageSize))
	if err != nil {
		return nil, 0, err
	}

	items := make([]HotKeywordItem, 0, len(rows))

	seen := make(map[string]struct{}, len(rows))

	for _, row := range rows {
		keyword := domain.NormalizeHotKeyword(row.Title)
		if keyword == "" {
			continue
		}

		if _, ok := seen[keyword]; ok {
			continue
		}

		seen[keyword] = struct{}{}
		items = append(items, HotKeywordItem{
			Keyword: keyword,
			Score:   row.Views,
		})
	}

	total := int64(len(items))

	start, end := domain.SliceWindow(len(items), pagination)
	if start == end {
		return []HotKeywordItem{}, total, nil
	}

	return items[start:end], total, nil
}
