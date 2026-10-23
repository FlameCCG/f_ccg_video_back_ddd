package search

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/search"

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
	relatedTagLimit = 10
)

// SearchService 站内搜索用例服务（原 service/search_service 的 Search/SearchVideos/SearchUsers）。
//
// ES 不可用或查询失败时回退到 MySQL 模糊查询，与原实现的降级路径逐条一致。
type SearchService struct {
	index    KeywordSearch
	fallback SearchFallbackQuery
	logger   *zap.Logger
}

// NewSearchService 构造站内搜索服务。
func NewSearchService(index KeywordSearch, fallback SearchFallbackQuery, logger *zap.Logger) *SearchService {
	return &SearchService{index: index, fallback: fallback, logger: logger}
}

// Search 同时查询视频/用户/标签三个维度（标签由视频命中派生）。
//
// 标签总量是本次视频命中派生的标签数，不是独立搜索整个标签表的总数。
// ES 超时后的 SQL 回退使用父 context，既避免复用已过期的子预算，也仍受请求总取消约束。
func (s *SearchService) Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	if !s.index.Available() {
		return s.fallbackSearch(ctx, opts)
	}

	page := domain.NormalizePagination(opts.Page, opts.PageSize)
	videoSort := domain.NormalizeVideoSort(opts.VideoSort, opts.VideoOrder)
	userSort := domain.NormalizeUserSort(opts.UserSort, opts.UserOrder)
	opts = applyNormalized(opts, page, videoSort, userSort)

	parent := ensureContext(ctx)

	esCtx, cancel := context.WithTimeout(parent, searchTimeout)
	defer cancel()

	hits, err := s.index.MultiSearch(esCtx, opts, page.Offset())
	if err != nil {
		// 响应条数异常属于契约错误（原实现同样直接报错，不回退）。
		if errors.Is(err, ErrUnexpectedMultiSearchSize) {
			return nil, err
		}

		s.logger.Warn("multi search fallback to mysql", zap.String("keyword", opts.Keyword), zap.Error(err))

		// 用未附加 ES 超时的父上下文回退，避免 ES 超时把回退查询一起拖死。
		return s.fallbackSearch(parent, opts)
	}

	result := &SearchResult{}
	// 保持原实现的 append 语义：零命中时 Videos/Users 仍为 nil（响应中的 null）。
	result.VideoTotal = hits.Videos.Total
	result.Videos = append(result.Videos, hits.Videos.Items...)
	result.UserTotal = hits.Users.Total
	result.Users = append(result.Users, hits.Users.Items...)

	s.hydrateVideoList(esCtx, result.Videos)

	result.Tags = buildRelatedTags(result.Videos)
	result.TagTotal = int64(len(result.Tags))
	result.Total = result.VideoTotal + result.UserTotal + result.TagTotal

	return result, nil
}

// SearchVideos 仅返回视频结果，同时返回用户总量（用于前端展示）。
func (s *SearchService) SearchVideos(ctx context.Context, opts SearchOptions) (*VideoSearchResult, error) {
	if !s.index.Available() {
		return s.fallbackSearchVideos(ctx, opts)
	}

	page := domain.NormalizePagination(opts.Page, opts.PageSize)
	videoSort := domain.NormalizeVideoSort(opts.VideoSort, opts.VideoOrder)
	opts.Page, opts.PageSize = page.Page, page.PageSize
	opts.VideoSort, opts.VideoOrder = videoSort.Field, videoSort.Order

	parent := ensureContext(ctx)

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
	page := domain.NormalizePagination(opts.Page, opts.PageSize)
	userSort := domain.NormalizeUserSort(opts.UserSort, opts.UserOrder)
	opts.Page, opts.PageSize = page.Page, page.PageSize
	opts.UserSort, opts.UserOrder = userSort.Field, userSort.Order

	parent := ensureContext(ctx)

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

// fallbackSearch 组合视频与用户回退结果，并派生相关标签。
func (s *SearchService) fallbackSearch(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	videoResult, err := s.fallbackSearchVideos(ctx, opts)
	if err != nil {
		return nil, err
	}

	userResult, err := s.fallbackSearchUsers(ctx, opts)
	if err != nil {
		return nil, err
	}

	result := &SearchResult{
		VideoTotal: videoResult.VideoTotal,
		UserTotal:  userResult.UserTotal,
		Videos:     videoResult.Videos,
		Users:      userResult.Users,
	}
	result.Tags = buildRelatedTags(result.Videos)
	result.TagTotal = int64(len(result.Tags))
	result.Total = result.VideoTotal + result.UserTotal + result.TagTotal

	return result, nil
}

func (s *SearchService) fallbackSearchVideos(ctx context.Context, opts SearchOptions) (*VideoSearchResult, error) {
	if !s.fallback.Available() {
		return &VideoSearchResult{Videos: []SearchVideoHit{}}, nil
	}

	ctx = ensureContext(ctx)

	if domain.NormalizeKeyword(opts.Keyword) == "" {
		return &VideoSearchResult{Videos: []SearchVideoHit{}}, nil
	}

	userTotal, err := s.fallback.CountUsers(ctx, opts.Keyword)
	if err != nil {
		return nil, err
	}

	videos, total, err := s.fallback.SearchVideos(ctx, opts)
	if err != nil {
		return nil, err
	}

	if len(videos) == 0 {
		videos = []SearchVideoHit{}
	}

	return &VideoSearchResult{
		VideoTotal: total,
		UserTotal:  userTotal,
		Videos:     videos,
	}, nil
}

func (s *SearchService) fallbackSearchUsers(ctx context.Context, opts SearchOptions) (*UserSearchResult, error) {
	if !s.fallback.Available() {
		return &UserSearchResult{Users: []SearchUserHit{}}, nil
	}

	ctx = ensureContext(ctx)

	if domain.NormalizeKeyword(opts.Keyword) == "" {
		return &UserSearchResult{Users: []SearchUserHit{}}, nil
	}

	users, total, err := s.fallback.SearchUsers(ctx, opts)
	if err != nil {
		return nil, err
	}

	if len(users) == 0 {
		users = []SearchUserHit{}
	}

	return &UserSearchResult{UserTotal: total, Users: users}, nil
}

// userTotalBestEffort 尽力取用户命中总量：优先 ES，失败退 MySQL COUNT，再失败记 0。
func (s *SearchService) userTotalBestEffort(ctx context.Context, keyword string) int64 {
	keyword = domain.NormalizeKeyword(keyword)
	if keyword == "" {
		return 0
	}

	ctx = ensureContext(ctx)

	if s.index.Available() {
		esCtx, cancel := context.WithTimeout(ctx, videoUserCountTimeout)

		total, err := s.index.CountUsers(esCtx, keyword)

		cancel()

		if err == nil {
			return total
		}

		s.logger.Warn("user total fallback to mysql", zap.String("keyword", keyword), zap.Error(err))
	}

	dbCtx, cancel := context.WithTimeout(ctx, videoUserCountTimeout)
	defer cancel()

	total, err := s.fallback.CountUsers(dbCtx, keyword)
	if err != nil {
		s.logger.Warn("user total count failed", zap.String("keyword", keyword), zap.Error(err))

		return 0
	}

	return total
}

// hydrateVideoList 用库内数据回填 ES 文档中缺失的作者名与创建时间。
func (s *SearchService) hydrateVideoList(ctx context.Context, hits []SearchVideoHit) {
	if len(hits) == 0 || !s.fallback.Available() {
		return
	}

	idSet := make(map[uint]struct{}, len(hits))

	ids := make([]uint, 0, len(hits))

	for _, item := range hits {
		if item.ID == 0 || (!item.CreatedAt.IsZero() && item.AuthorUsername != "") {
			continue
		}

		if _, ok := idSet[item.ID]; ok {
			continue
		}

		idSet[item.ID] = struct{}{}
		ids = append(ids, item.ID)
	}

	if len(ids) == 0 {
		return
	}

	rows, err := s.fallback.VideoMetaByIDs(ctx, ids)
	if err != nil {
		s.logger.Warn("hydrate search videos failed", zap.Error(err))

		return
	}

	metaByID := make(map[uint]VideoSearchMeta, len(rows))
	for _, row := range rows {
		metaByID[row.ID] = row
	}

	applyVideoMeta(hits, metaByID)
}

// applyVideoMeta 就地回填非空的作者名与创建时间。
func applyVideoMeta(hits []SearchVideoHit, metaByID map[uint]VideoSearchMeta) {
	for i := range hits {
		meta, ok := metaByID[hits[i].ID]
		if !ok {
			continue
		}

		if meta.AuthorUsername != "" {
			hits[i].AuthorUsername = meta.AuthorUsername
		}

		if !meta.CreatedAt.IsZero() {
			hits[i].CreatedAt = meta.CreatedAt
		}
	}
}

// buildRelatedTags 从视频命中的标签中去重派生相关标签（最多 10 个）。
func buildRelatedTags(videos []SearchVideoHit) []SearchTagHit {
	seen := make(map[string]struct{})
	tags := make([]SearchTagHit, 0)

	for _, video := range videos {
		for _, tag := range video.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}

			if _, ok := seen[tag]; ok {
				continue
			}

			seen[tag] = struct{}{}

			tags = append(tags, SearchTagHit{Name: tag})

			if len(tags) >= relatedTagLimit {
				return tags
			}
		}
	}

	return tags
}

// applyNormalized 把规范化后的分页与排序写回搜索参数。
func applyNormalized(opts SearchOptions, page domain.Pagination, videoSort, userSort domain.SortSpec) SearchOptions {
	opts.Page, opts.PageSize = page.Page, page.PageSize
	opts.VideoSort, opts.VideoOrder = videoSort.Field, videoSort.Order
	opts.UserSort, opts.UserOrder = userSort.Field, userSort.Order

	return opts
}

// ensureContext 保证向下游传递非 nil 上下文（与原 ctx == nil 兜底一致）。
func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.TODO()
	}

	return ctx
}
