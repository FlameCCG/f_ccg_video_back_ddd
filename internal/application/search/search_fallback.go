package search

import (
	"context"

	"go.uber.org/zap"
)

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
	result.Tags = s.buildRelatedTags(result.Videos)
	result.TagTotal = int64(len(result.Tags))
	result.Total = result.VideoTotal + result.UserTotal + result.TagTotal

	return result, nil
}

func (s *SearchService) fallbackSearchVideos(ctx context.Context, opts SearchOptions) (*VideoSearchResult, error) {
	if !s.fallback.Available() {
		return &VideoSearchResult{Videos: []SearchVideoHit{}}, nil
	}

	ctx = (callContext{}).ensure(ctx)

	if s.keywords.NormalizeKeyword(opts.Keyword) == "" {
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

	ctx = (callContext{}).ensure(ctx)

	if s.keywords.NormalizeKeyword(opts.Keyword) == "" {
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
	keyword = s.keywords.NormalizeKeyword(keyword)
	if keyword == "" {
		return 0
	}

	ctx = (callContext{}).ensure(ctx)

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
