package search

import (
	"context"
	"f_ccg_video_back_ddd/pkg/logctx"
	"strconv"
	"strings"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/search"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// suggestPrimaryTimeout 联想主查询超时（整体与单个索引查询共用同一预算）.
const suggestPrimaryTimeout = 800 * time.Millisecond

// SuggestService 搜索联想用例服务（原 service/search_service.Suggest）。
//
// 基于现有 text/keyword/pinyin 倒排索引做联想，不依赖 completion FST：
// 零额外空间、无需 reindex、支持单字/中文/拼音/中间片段（如 ce6 命中 毛民_ce6ffa…）。
// 优先级：title > username > tag。
type SuggestService struct {
	index    SuggestSearch
	fallback SuggestFallbackQuery
	cache    SuggestionCache
	logger   *zap.Logger
}

// NewSuggestService 构造搜索联想服务。
func NewSuggestService(
	index SuggestSearch,
	fallback SuggestFallbackQuery,
	cache SuggestionCache,
	logger *zap.Logger,
) *SuggestService {
	return &SuggestService{index: index, fallback: fallback, cache: cache, logger: logger}
}

// Suggest 返回联想候选项；ES 不可用或查询失败时回退 MySQL LIKE 查询。
func (s *SuggestService) Suggest(ctx context.Context, prefix string, size int) ([]SuggestionItem, error) {
	size = domain.NormalizeSuggestSize(size)

	prefix = domain.NormalizeKeyword(prefix)
	if prefix == "" {
		return []SuggestionItem{}, nil
	}

	cacheKey := suggestCacheKey(prefix, size)
	if cached, ok := s.cache.Load(cacheKey); ok {
		return cached, nil
	}

	if !s.index.Available() {
		items, err := s.fallbackSuggest(ctx, prefix, size)
		if err != nil {
			return nil, err
		}

		s.cache.Store(cacheKey, items)

		return items, nil
	}

	parent := ensureContext(ctx)

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
		logctx.With(ctx, s.logger).Warn("suggest fallback to mysql", zap.String("event", "search.suggest_service.suggest.degraded"), zap.String("operation", "search.suggest_service.suggest"), zap.String("prefix", prefix), zap.Error(err))

		items, fallbackErr := s.fallbackSuggest(parent, prefix, size)
		if fallbackErr != nil {
			return nil, fallbackErr
		}

		s.cache.Store(cacheKey, items)

		return items, nil
	}

	result := mergeSuggestionPriority(size, videoItems, userItems, tagItems)
	s.cache.Store(cacheKey, result)

	return result, nil
}

// fetch 取单个维度的联想候选，并为该次查询单独设定超时。
func (s *SuggestService) fetch(parent context.Context, docType, prefix string, size int) ([]SuggestionItem, error) {
	if size <= 0 {
		return []SuggestionItem{}, nil
	}

	ctx, cancel := context.WithTimeout(ensureContext(parent), suggestPrimaryTimeout)
	defer cancel()

	return s.index.FieldSuggest(ctx, docType, prefix, size)
}

// fallbackSuggest 依次用视频标题、用户名、标签名补齐候选（数量不足才查下一个维度）。
func (s *SuggestService) fallbackSuggest(ctx context.Context, prefix string, size int) ([]SuggestionItem, error) {
	if !s.fallback.Available() || size <= 0 {
		return []SuggestionItem{}, nil
	}

	ctx = ensureContext(ctx)

	prefix = domain.NormalizeKeyword(prefix)
	if prefix == "" {
		return []SuggestionItem{}, nil
	}

	result := make([]SuggestionItem, 0, size)
	seen := make(map[string]struct{}, size)

	appendItems := func(items []SuggestionItem) {
		for _, item := range items {
			key := item.DocType + "|" + item.Value
			if _, ok := seen[key]; ok {
				continue
			}

			seen[key] = struct{}{}

			result = append(result, item)
			if len(result) >= size {
				return
			}
		}
	}

	titles, err := s.fallback.VideoTitles(ctx, prefix, size)
	if err != nil {
		return nil, err
	}

	appendItems(buildSuggestionItems(titles, prefix, DocTypeVideo))

	if len(result) < size {
		usernames, err := s.fallback.Usernames(ctx, prefix, size-len(result))
		if err != nil {
			return nil, err
		}

		appendItems(buildSuggestionItems(usernames, prefix, DocTypeUser))
	}

	if len(result) < size {
		names, err := s.fallback.TagNames(ctx, prefix, size-len(result))
		if err != nil {
			return nil, err
		}

		appendItems(buildSuggestionItems(names, prefix, DocTypeTag))
	}

	return result, nil
}

// buildSuggestionItems 把候选值列表包装为带高亮的联想项（丢弃空值）。
func buildSuggestionItems(values []string, prefix, docType string) []SuggestionItem {
	items := make([]SuggestionItem, 0, len(values))

	for _, value := range values {
		if value == "" {
			continue
		}

		items = append(items, SuggestionItem{
			Value:     value,
			Highlight: domain.HighlightSuggestion(value, prefix),
			DocType:   docType,
		})
	}

	return items
}

// mergeSuggestionPriority 按分组顺序合并候选并按「类型+小写值」去重，最多取 size 条。
func mergeSuggestionPriority(size int, groups ...[]SuggestionItem) []SuggestionItem {
	result := make([]SuggestionItem, 0, size)
	seen := make(map[string]struct{}, size)

	for _, group := range groups {
		for _, item := range group {
			key := item.DocType + "|" + strings.ToLower(item.Value)
			if _, ok := seen[key]; ok {
				continue
			}

			seen[key] = struct{}{}

			result = append(result, item)

			if len(result) >= size {
				return result
			}
		}
	}

	return result
}

// suggestCacheKey 生成联想缓存键（小写前缀 + 候选数）。
func suggestCacheKey(prefix string, size int) string {
	return domain.LowerKeyword(prefix) + "|" + strconv.Itoa(size)
}
