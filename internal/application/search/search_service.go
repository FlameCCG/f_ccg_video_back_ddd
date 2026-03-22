package search

import (
	"context"
	"errors"
	"time"

	searchdomain "f_ccg_video_back_ddd/internal/domain/search"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	// searchTimeout 搜索接口整体超时.
	searchTimeout = 5 * time.Second
	// videoSearchPrimaryTimeout 视频主查询超时.
	videoSearchPrimaryTimeout = 1200 * time.Millisecond
	// videoUserCountTimeout 视频搜索附带的用户总量查询超时.
	videoUserCountTimeout = 500 * time.Millisecond
	// userSearchPrimaryTimeout 用户主查询超时.
	userSearchPrimaryTimeout = 1200 * time.Millisecond
	// relatedTagLimit 综合搜索派生标签数量上限.
	relatedTagLimit = searchdomain.RelatedTagLimit
)

// SearchService 站内搜索用例服务。
//
// ES 不可用或查询失败时回退到 MySQL 模糊查询。
type SearchService struct {
	index    KeywordSearch
	fallback SearchFallbackQuery
	logger   *zap.Logger
	keywords searchdomain.KeywordPolicy
	pages    searchdomain.PaginationPolicy
}

// NewSearchService 构造站内搜索服务。
func NewSearchService(index KeywordSearch, fallback SearchFallbackQuery, logger *zap.Logger) *SearchService {
	return &SearchService{
		index:    index,
		fallback: fallback,
		logger:   logger,
		keywords: searchdomain.KeywordPolicy{},
		pages:    searchdomain.PaginationPolicy{},
	}
}

// Search 同时查询视频/用户/标签三个维度（标签由视频命中派生）。
//
// 标签总量是本次视频命中派生的标签数，不是独立搜索整个标签表的总数。
// ES 超时后的 SQL 回退使用父 context，既避免复用已过期的子预算，也仍受请求总取消约束。
func (s *SearchService) Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	page := s.pages.NormalizePagination(opts.Page, opts.PageSize)
	videoSort := s.pages.NormalizeVideoSort(opts.VideoSort, opts.VideoOrder)
	userSort := s.pages.NormalizeUserSort(opts.UserSort, opts.UserOrder)
	opts = s.applyNormalized(opts, page, videoSort, userSort)

	if !s.index.Available() {
		return s.fallbackSearch(ctx, opts)
	}

	parent := (callContext{}).ensure(ctx)

	esCtx, cancel := context.WithTimeout(parent, searchTimeout)
	defer cancel()

	hits, err := s.index.MultiSearch(esCtx, opts, page.Offset())
	if err != nil {
		// 响应条数异常属于契约错误。
		if errors.Is(err, ErrUnexpectedMultiSearchSize) {
			return nil, err
		}

		s.logger.Warn("multi search fallback to mysql", zap.String("keyword", opts.Keyword), zap.Error(err))

		// 用未附加 ES 超时的父上下文回退，避免 ES 超时把回退查询一起拖死。
		return s.fallbackSearch(parent, opts)
	}

	result := &SearchResult{}
	// 零命中时 Videos/Users 为 nil，响应 JSON 输出 null。
	result.VideoTotal = hits.Videos.Total
	result.Videos = append(result.Videos, hits.Videos.Items...)
	result.UserTotal = hits.Users.Total
	result.Users = append(result.Users, hits.Users.Items...)

	s.hydrateVideoList(esCtx, result.Videos)

	result.Tags = s.buildRelatedTags(result.Videos)
	result.TagTotal = int64(len(result.Tags))
	result.Total = result.VideoTotal + result.UserTotal + result.TagTotal

	return result, nil
}

// SearchVideos 仅返回视频结果，同时返回用户总量（用于前端展示）。
func (s *SearchService) SearchVideos(ctx context.Context, opts SearchOptions) (*VideoSearchResult, error) {
	page := s.pages.NormalizePagination(opts.Page, opts.PageSize)
	videoSort := s.pages.NormalizeVideoSort(opts.VideoSort, opts.VideoOrder)
	opts.Page, opts.PageSize = page.Page, page.PageSize
	opts.VideoSort, opts.VideoOrder = videoSort.Field, videoSort.Order

	if !s.index.Available() {
		return s.fallbackSearchVideos(ctx, opts)
	}

	parent := (callContext{}).ensure(ctx)

	esCtx, cancel := context.WithTimeout(parent, searchTimeout)
	defer cancel()

	var (
		videoHits VideoHits
		userTotal int64
	)

	group, groupCtx := errgroup.WithContext(esCtx)
	group.Go(func() error {
		videoCtx, cancelVideo := context.WithTimeout(groupCtx, videoSearchPrimaryTimeout)
		defer cancelVideo()

		hits, err := s.index.SearchVideos(videoCtx, opts, page.Offset())
		if err != nil {
			return err
		}

		videoHits = hits

		return nil
	})
	group.Go(func() error {
		userTotal = s.userTotalBestEffort(groupCtx, opts.Keyword)

		return nil
	})

	if err := group.Wait(); err != nil {
		s.logger.Warn("video search fallback to mysql", zap.String("keyword", opts.Keyword), zap.Error(err))

		return s.fallbackSearchVideos(parent, opts)
	}

	result := &VideoSearchResult{UserTotal: userTotal}
	result.VideoTotal = videoHits.Total
	result.Videos = videoHits.Items
	s.hydrateVideoList(esCtx, result.Videos)

	return result, nil
}

// SearchUsers 仅返回用户结果。
func (s *SearchService) SearchUsers(ctx context.Context, opts SearchOptions) (*UserSearchResult, error) {
	page := s.pages.NormalizePagination(opts.Page, opts.PageSize)
	userSort := s.pages.NormalizeUserSort(opts.UserSort, opts.UserOrder)
	opts.Page, opts.PageSize = page.Page, page.PageSize
	opts.UserSort, opts.UserOrder = userSort.Field, userSort.Order

	parent := (callContext{}).ensure(ctx)

	if !s.index.Available() {
		return s.fallbackSearchUsers(parent, opts)
	}

	esCtx, cancel := context.WithTimeout(parent, userSearchPrimaryTimeout)
	defer cancel()

	hits, err := s.index.SearchUsers(esCtx, opts, page.Offset())
	if err != nil {
		s.logger.Warn("user search fallback to mysql", zap.String("keyword", opts.Keyword), zap.Error(err))

		return s.fallbackSearchUsers(parent, opts)
	}

	return &UserSearchResult{UserTotal: hits.Total, Users: hits.Items}, nil
}
