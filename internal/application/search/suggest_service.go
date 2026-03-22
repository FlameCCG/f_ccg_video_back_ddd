package search

import (
	"context"
	"time"

	searchdomain "f_ccg_video_back_ddd/internal/domain/search"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// suggestPrimaryTimeout 联想主查询超时（整体与单个索引查询共用同一预算）.
const suggestPrimaryTimeout = 800 * time.Millisecond

// SuggestService 搜索联想用例服务。
//
// 基于现有 text/keyword/pinyin 倒排索引做联想，不依赖 completion FST：
// 零额外空间、无需 reindex、支持单字/中文/拼音/中间片段（如 ce6 命中 毛民_ce6ffa…）。
// 优先级：title > username > tag。
type SuggestService struct {
	index    SuggestSearch
	fallback SuggestFallbackQuery
	cache    SuggestionCache
	logger   *zap.Logger
	keywords searchdomain.KeywordPolicy
	pages    searchdomain.PaginationPolicy
}

// NewSuggestService 构造搜索联想服务。
func NewSuggestService(
	index SuggestSearch,
	fallback SuggestFallbackQuery,
	cache SuggestionCache,
	logger *zap.Logger,
) *SuggestService {
	return &SuggestService{
		index:    index,
		fallback: fallback,
		cache:    cache,
		logger:   logger,
		keywords: searchdomain.KeywordPolicy{},
		pages:    searchdomain.PaginationPolicy{},
	}
}

// Suggest 返回联想候选项；ES 不可用或查询失败时回退 MySQL LIKE 查询。
func (s *SuggestService) Suggest(ctx context.Context, prefix string, size int) ([]SuggestionItem, error) {
	size = s.pages.NormalizeSuggestSize(size)

	prefix = s.keywords.NormalizeKeyword(prefix)
	if prefix == "" {
		return []SuggestionItem{}, nil
	}

	if cached, ok := s.cache.Load(prefix, size); ok {
		return cached, nil
	}

	if !s.index.Available() {
		items, err := s.fallbackSuggest(ctx, prefix, size)
		if err != nil {
			return nil, err
		}

		s.cache.Store(prefix, size, items)

		return items, nil
	}

	parent := (callContext{}).ensure(ctx)

	suggestCtx, cancel := context.WithTimeout(parent, suggestPrimaryTimeout)
	defer cancel()

	var (
		videoItems []SuggestionItem
		userItems  []SuggestionItem
		tagItems   []SuggestionItem
	)

	group, groupCtx := errgroup.WithContext(suggestCtx)
	group.Go(func() error {
		items, err := s.fetch(groupCtx, DocTypeVideo, prefix, size)
		if err != nil {
			return err
		}

		videoItems = items

		return nil
	})
	group.Go(func() error {
		items, err := s.fetch(groupCtx, DocTypeUser, prefix, size)
		if err != nil {
			return err
		}

		userItems = items

		return nil
	})
	group.Go(func() error {
		items, err := s.fetch(groupCtx, DocTypeTag, prefix, size)
		if err != nil {
			return err
		}

		tagItems = items

		return nil
	})

	if err := group.Wait(); err != nil {
		s.logger.Warn("suggest fallback to mysql", zap.String("prefix", prefix), zap.Error(err))

		items, fallbackErr := s.fallbackSuggest(parent, prefix, size)
		if fallbackErr != nil {
			return nil, fallbackErr
		}

		s.cache.Store(prefix, size, items)

		return items, nil
	}

	result := s.mergeSuggestionPriority(size, videoItems, userItems, tagItems)
	s.cache.Store(prefix, size, result)

	return result, nil
}

// fetch 取单个维度的联想候选，并为该次查询单独设定超时。
func (s *SuggestService) fetch(parent context.Context, docType, prefix string, size int) ([]SuggestionItem, error) {
	if size <= 0 {
		return []SuggestionItem{}, nil
	}

	ctx, cancel := context.WithTimeout((callContext{}).ensure(parent), suggestPrimaryTimeout)
	defer cancel()

	return s.index.FieldSuggest(ctx, docType, prefix, size)
}

// fallbackSuggest 依次用视频标题、用户名、标签名补齐候选（数量不足才查下一个维度）。
func (s *SuggestService) fallbackSuggest(ctx context.Context, prefix string, size int) ([]SuggestionItem, error) {
	if !s.fallback.Available() || size <= 0 {
		return []SuggestionItem{}, nil
	}

	ctx = (callContext{}).ensure(ctx)

	prefix = s.keywords.NormalizeKeyword(prefix)
	if prefix == "" {
		return []SuggestionItem{}, nil
	}

	result := make([]SuggestionItem, 0, size)
	appendItems := func(items []SuggestionItem) {
		result = s.selectSuggestions(size, false, result, items)
	}

	titles, err := s.fallback.VideoTitles(ctx, prefix, size)
	if err != nil {
		return nil, err
	}

	appendItems(s.buildSuggestionItems(titles, prefix, DocTypeVideo))

	if len(result) < size {
		usernames, err := s.fallback.Usernames(ctx, prefix, size-len(result))
		if err != nil {
			return nil, err
		}

		appendItems(s.buildSuggestionItems(usernames, prefix, DocTypeUser))
	}

	if len(result) < size {
		names, err := s.fallback.TagNames(ctx, prefix, size-len(result))
		if err != nil {
			return nil, err
		}

		appendItems(s.buildSuggestionItems(names, prefix, DocTypeTag))
	}

	return result, nil
}

// buildSuggestionItems 把候选值列表包装为带高亮的联想项（丢弃空值）。
func (s *SuggestService) buildSuggestionItems(values []string, prefix, docType string) []SuggestionItem {
	items := make([]SuggestionItem, 0, len(values))

	for _, value := range values {
		if value == "" {
			continue
		}

		items = append(items, SuggestionItem{
			Value:     value,
			Highlight: (SuggestionHighlighter{}).HighlightSuggestion(value, prefix),
			DocType:   docType,
		})
	}

	return items
}

// mergeSuggestionPriority 按分组顺序合并候选并按「类型+小写值」去重，最多取 size 条。
func (s *SuggestService) mergeSuggestionPriority(size int, groups ...[]SuggestionItem) []SuggestionItem {
	return s.selectSuggestions(size, true, groups...)
}

// selectSuggestions 只转换候选事实和响应，高亮内容不进入领域。
func (s *SuggestService) selectSuggestions(size int, foldCase bool, groups ...[]SuggestionItem) []SuggestionItem {
	facts := make([][]searchdomain.SuggestionCandidate, len(groups))
	for group, items := range groups {
		facts[group] = make([]searchdomain.SuggestionCandidate, len(items))
		for i, item := range items {
			facts[group][i] = searchdomain.SuggestionCandidate{Kind: item.DocType, Value: item.Value}
		}
	}
	selected := (searchdomain.SuggestionPolicy{}).Select(size, foldCase, facts...)
	result := make([]SuggestionItem, 0, len(selected))
	for _, position := range selected {
		result = append(result, groups[position.Group][position.Index])
	}
	return result
}
