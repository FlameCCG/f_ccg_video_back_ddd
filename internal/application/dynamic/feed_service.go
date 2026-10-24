package dynamic

import (
	"context"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/dynamic"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// feedMetricTimeout 作品流补充指标（点赞状态、评论数）的单项查询超时。
//
// 这些指标是「尽力而为」的：超时或失败只记录日志并按空值渲染，不影响主列表返回。
const feedMetricTimeout = 800 * time.Millisecond

// FeedService 作品流查询用例服务（原 service/dynamic_service/feed_service.go）。
type FeedService struct {
	users  UserProfileQuery
	feed   FeedQuery
	logger *zap.Logger
}

// NewFeedService 构造作品流查询用例服务。
func NewFeedService(users UserProfileQuery, feed FeedQuery, logger *zap.Logger) *FeedService {
	return &FeedService{users: users, feed: feed, logger: logger}
}

// ListFeed 分页查询作品流（视频 + 动态）。
//
// workType：0 全部、1 仅视频、2 仅动态；authorID=0 时返回浏览者本人及关注用户的聚合流。
func (s *FeedService) ListFeed(
	ctx context.Context,
	userID, authorID uint,
	workType, page, pageSize int,
) ([]WorkFeedItem, int64, error) {
	page, pageSize = domain.NormalizeFeedPaging(page, pageSize)
	offset := max((page-1)*pageSize, 0)

	if authorID > 0 {
		return s.listAuthorFeed(ctx, userID, authorID, workType, offset, pageSize)
	}

	if userID == 0 {
		return nil, 0, domain.ErrUserNotLogin
	}

	return s.listFollowingFeed(ctx, userID, workType, offset, pageSize)
}

func (s *FeedService) listFollowingFeed(
	ctx context.Context,
	viewerID uint,
	workType, offset, limit int,
) ([]WorkFeedItem, int64, error) {
	sources := domain.BuildFeedSources(workType)
	if !sources.Any() {
		return []WorkFeedItem{}, 0, nil
	}

	rows, err := s.feed.ListFollowingWorks(ctx, FollowingFeedParams{
		ViewerID: viewerID,
		Sources:  sources,
		Offset:   offset,
		Limit:    limit + 1,
	})
	if err != nil {
		return nil, 0, err
	}

	total, rows, err := resolveFeedTotal(ctx, rows, offset, limit, func(countCtx context.Context) (int64, error) {
		return s.feed.CountFollowingWorks(countCtx, FollowingFeedCountParams{
			ViewerID: viewerID,
			Sources:  sources,
		})
	})
	if err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []WorkFeedItem{}, 0, nil
	}

	metrics, err := s.buildFeedMetrics(ctx, viewerID, rows)
	if err != nil {
		return nil, 0, err
	}

	items := make([]WorkFeedItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, rowToFeedItem(row, metrics))
	}

	return items, total, nil
}

func (s *FeedService) listAuthorFeed(
	ctx context.Context,
	viewerID, authorID uint,
	workType, offset, limit int,
) ([]WorkFeedItem, int64, error) {
	author, err := s.users.ProfileByID(ctx, authorID)
	if err != nil {
		return nil, 0, err
	}

	if author == nil {
		return nil, 0, domain.ErrRecordNotFound
	}

	isOwnerView := viewerID > 0 && viewerID == authorID

	sources := domain.BuildFeedSources(workType)
	if !sources.Any() {
		return []WorkFeedItem{}, 0, nil
	}

	// 多取一行用于探测是否存在下一页：不足 limit+1 行时总数可直接由偏移推出。
	rows, err := s.feed.ListAuthorWorks(ctx, AuthorFeedParams{
		AuthorID:       author.ID,
		AuthorUsername: author.Username,
		AuthorAvatar:   author.Avatar,
		Sources:        sources,
		IsOwnerView:    isOwnerView,
		Offset:         offset,
		Limit:          limit + 1,
	})
	if err != nil {
		return nil, 0, err
	}

	total, rows, err := resolveFeedTotal(ctx, rows, offset, limit, func(countCtx context.Context) (int64, error) {
		return s.feed.CountAuthorWorks(countCtx, AuthorFeedCountParams{
			AuthorID:       authorID,
			AuthorUsername: author.Username,
			Sources:        sources,
			IsOwnerView:    isOwnerView,
		})
	})
	if err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []WorkFeedItem{}, 0, nil
	}

	metrics, err := s.buildFeedMetrics(ctx, viewerID, rows)
	if err != nil {
		return nil, 0, err
	}

	items := make([]WorkFeedItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, rowToFeedItem(r, metrics))
	}

	return items, total, nil
}

// feedMetrics 作品流补充指标集合。
type feedMetrics struct {
	videoLikeCounts    map[uint]int64
	videoCommentCounts map[uint]int64
	dynamicLikeCounts  map[uint]int64
	dynamicComments    map[uint]int64
	likedVideoIDs      map[uint]bool
	likedDynamicIDs    map[uint]bool
}

// buildFeedMetrics 汇总作品流的点赞数、评论数与浏览者点赞状态。
//
// 视频/动态的点赞数直接取自联合查询结果；评论数与点赞状态并发补齐（尽力而为）。
func (s *FeedService) buildFeedMetrics(ctx context.Context, viewerID uint, rows []FeedRow) (*feedMetrics, error) {
	metrics := &feedMetrics{
		videoLikeCounts:    map[uint]int64{},
		videoCommentCounts: map[uint]int64{},
		dynamicLikeCounts:  map[uint]int64{},
		dynamicComments:    map[uint]int64{},
		likedVideoIDs:      map[uint]bool{},
		likedDynamicIDs:    map[uint]bool{},
	}
	if len(rows) == 0 {
		return metrics, nil
	}

	videoIDs := collectFeedIDs(rows, enum.WorkTypeVideo)
	dynamicIDs := collectFeedIDs(rows, enum.WorkTypeDynamic)

	var (
		dynamicComments map[uint]int64
		likedVideoIDs   map[uint]bool
		likedDynamicIDs map[uint]bool
	)

	group, groupCtx := errgroup.WithContext(ctx)

	for _, row := range rows {
		if enum.WorkType(row.WorkType) == enum.WorkTypeVideo {
			metrics.videoLikeCounts[row.WorkID] = row.VideoLikeCount
			metrics.videoCommentCounts[row.WorkID] = row.VideoComment
		}
	}

	if len(videoIDs) > 0 && viewerID > 0 {
		group.Go(func() error {
			likedVideoIDs = s.likedVideoIDsBestEffort(groupCtx, viewerID, videoIDs)

			return nil
		})
	}

	if len(dynamicIDs) > 0 {
		for _, row := range rows {
			if enum.WorkType(row.WorkType) != enum.WorkTypeDynamic {
				continue
			}

			metrics.dynamicLikeCounts[row.WorkID] = row.DynLikeCount
		}

		group.Go(func() error {
			dynamicComments = s.dynamicCommentsBestEffort(groupCtx, dynamicIDs)

			return nil
		})

		if viewerID > 0 {
			group.Go(func() error {
				likedDynamicIDs = s.likedDynamicIDsBestEffort(groupCtx, viewerID, dynamicIDs)

				return nil
			})
		}
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}

	if dynamicComments != nil {
		metrics.dynamicComments = dynamicComments
	}

	if likedVideoIDs != nil {
		metrics.likedVideoIDs = likedVideoIDs
	}

	if likedDynamicIDs != nil {
		metrics.likedDynamicIDs = likedDynamicIDs
	}

	return metrics, nil
}

// collectFeedIDs 从联合查询结果中按作品类型去重收集作品 ID。
func collectFeedIDs(rows []FeedRow, workType enum.WorkType) []uint {
	if len(rows) == 0 {
		return nil
	}

	seen := make(map[uint]struct{}, len(rows))

	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		if enum.WorkType(r.WorkType) != workType || r.WorkID == 0 {
			continue
		}

		if _, ok := seen[r.WorkID]; ok {
			continue
		}

		seen[r.WorkID] = struct{}{}
		ids = append(ids, r.WorkID)
	}

	return ids
}

// resolveFeedTotal 由「多取一行」的探测结果推导总数：
// 未触及下一页时总数即偏移 + 当前行数，可省去一次 COUNT。
func resolveFeedTotal(
	ctx context.Context,
	rows []FeedRow,
	offset, limit int,
	countFn func(context.Context) (int64, error),
) (int64, []FeedRow, error) {
	if len(rows) == 0 {
		if offset > 0 && countFn != nil {
			total, err := countFn(ctx)
			return total, []FeedRow{}, err
		}
		return 0, []FeedRow{}, nil
	}

	if len(rows) <= limit {
		return int64(offset + len(rows)), rows, nil
	}

	rows = rows[:limit]
	if countFn == nil {
		return int64(offset + len(rows)), rows, nil
	}

	total, err := countFn(ctx)
	if err != nil {
		return 0, nil, err
	}

	return total, rows, nil
}

func (s *FeedService) likedVideoIDsBestEffort(ctx context.Context, viewerID uint, videoIDs []uint) map[uint]bool {
	result := map[uint]bool{}
	if viewerID == 0 || len(videoIDs) == 0 {
		return result
	}

	queryCtx, cancel := context.WithTimeout(ctx, feedMetricTimeout)
	defer cancel()

	ids, err := s.feed.LikedVideoIDs(queryCtx, viewerID, videoIDs)
	if err != nil {
		s.logger.Warn("load liked video ids failed", zap.Uint("viewerId", viewerID), zap.Error(err))

		return result
	}

	return ids
}

func (s *FeedService) likedDynamicIDsBestEffort(ctx context.Context, viewerID uint, dynamicIDs []uint) map[uint]bool {
	result := map[uint]bool{}
	if viewerID == 0 || len(dynamicIDs) == 0 {
		return result
	}

	queryCtx, cancel := context.WithTimeout(ctx, feedMetricTimeout)
	defer cancel()

	ids, err := s.feed.LikedDynamicIDs(queryCtx, viewerID, dynamicIDs)
	if err != nil {
		s.logger.Warn("load liked dynamic ids failed", zap.Uint("viewerId", viewerID), zap.Error(err))

		return result
	}

	return ids
}

func (s *FeedService) dynamicCommentsBestEffort(ctx context.Context, dynamicIDs []uint) map[uint]int64 {
	result := map[uint]int64{}
	if len(dynamicIDs) == 0 {
		return result
	}

	queryCtx, cancel := context.WithTimeout(ctx, feedMetricTimeout)
	defer cancel()

	counts, err := s.feed.CommentCountsByDynamic(queryCtx, dynamicIDs)
	if err != nil {
		s.logger.Warn("load dynamic comment counts failed", zap.Error(err))

		return result
	}

	return counts
}

// rowToFeedItem 把联合查询行渲染为作品流条目。
func rowToFeedItem(r FeedRow, metrics *feedMetrics) WorkFeedItem {
	item := WorkFeedItem{
		WorkType:  enum.WorkType(r.WorkType),
		CreatedAt: r.CreatedAt,
		Author: AuthorBrief{
			ID:       r.AuthorID,
			Username: r.AuthorUsername,
			Avatar:   r.AuthorAvatar,
		},
	}
	if item.WorkType == enum.WorkTypeVideo {
		item.Video = &VideoBrief{
			ID:           r.WorkID,
			Title:        r.VideoTitle,
			Description:  r.VideoDesc,
			Cover:        r.VideoCover,
			Duration:     r.VideoDuration,
			Views:        r.VideoViews,
			DanmuCount:   r.VideoDanmu,
			LikeCount:    metrics.videoLikeCounts[r.WorkID],
			CommentCount: metrics.videoCommentCounts[r.WorkID],
			IsLiked:      metrics.likedVideoIDs[r.WorkID],
			IsPinned:     r.DynIsPinned,
			PinnedAt:     r.DynPinnedAt,
		}

		return item
	}

	item.Dynamic = &DynamicBrief{
		ID:           r.WorkID,
		Content:      r.DynContent,
		ImageURL:     r.DynImageURL,
		LikeCount:    metrics.dynamicLikeCounts[r.WorkID],
		CommentCount: metrics.dynamicComments[r.WorkID],
		IsLiked:      metrics.likedDynamicIDs[r.WorkID],
		IsPinned:     r.DynIsPinned,
		PinnedAt:     r.DynPinnedAt,
	}

	return item
}
