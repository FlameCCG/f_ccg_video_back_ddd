package comment

import (
	"context"
	"strings"

	commentdomain "f_ccg_video_back_ddd/internal/domain/comment"

	"golang.org/x/sync/errgroup"
)

// List 获取作品的根评论列表（分页）。
func (s *QueryService) List(ctx context.Context, params ListParams) ([]CommentItem, int64, error) {
	if _, err := (commentdomain.Draft{VideoID: params.VideoID, DynamicID: params.DynamicID}).TargetType(); err != nil {
		return nil, 0, err
	}

	page, pageSize := s.pages.Normalize(params.Page, params.PageSize)

	rows, total, err := s.query.ListRoots(ctx, RootListQuery{
		VideoID:   params.VideoID,
		DynamicID: params.DynamicID,
		Page:      page,
		PageSize:  pageSize,
		SortBy:    params.SortBy,
	})
	if err != nil {
		return nil, 0, err
	}

	return s.buildCommentItems(ctx, rows, params.ViewerID), total, nil
}

// Replies 获取某条评论的回复列表（分页）。
func (s *QueryService) Replies(ctx context.Context, params RepliesParams) ([]CommentItem, int64, error) {
	page, pageSize := s.pages.Normalize(params.Page, params.PageSize)

	rows, total, err := s.query.ListReplies(ctx, ReplyListQuery{
		RootID:   params.RootID,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, 0, err
	}

	return s.buildCommentItems(ctx, rows, params.ViewerID), total, nil
}

// ListByAuthor 创作中心查看本人视频的评论。
func (s *QueryService) ListByAuthor(ctx context.Context, params CreatorListParams) ([]CommentItem, int64, error) {
	if err := s.creators.RequireAuthor(params.AuthorID); err != nil {
		return nil, 0, err
	}

	page, pageSize := s.pages.Normalize(params.Page, params.PageSize)

	username := s.related.UsernameByID(ctx, params.AuthorID)
	if username == "" {
		return nil, 0, commentdomain.ErrAuthorNotFound
	}

	rows, total, err := s.query.ListByAuthor(ctx, CreatorListQuery{
		AuthorUsername: username,
		Page:           page,
		PageSize:       pageSize,
		Sort:           params.Sort,
		Keyword:        strings.TrimSpace(params.Keyword),
	})
	if err != nil {
		return nil, 0, err
	}

	return s.buildCommentItems(ctx, rows, params.AuthorID), total, nil
}

// GetCount 获取视频评论总数（含回复）。
func (s *QueryService) GetCount(ctx context.Context, videoID uint) (int64, error) {
	return s.query.CountByVideo(ctx, videoID)
}

// buildCommentItems 构建评论返回结构：叠加点赞数/回复数缓存与当前用户点赞状态。
func (s *QueryService) buildCommentItems(ctx context.Context, rows []CommentRow, userID uint) []CommentItem {
	if len(rows) == 0 {
		return []CommentItem{}
	}

	commentIDs := make([]uint, len(rows))
	for i, r := range rows {
		commentIDs[i] = r.ID
	}

	var (
		likedMap       map[uint]bool
		likedMissIDs   []uint
		countMap       map[uint]int64
		countMissIDs   []uint
		replyCountMap  map[uint]int64
		rebuildComment []uint
	)

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		likedMap, likedMissIDs = s.likeCache.LikedFlags(groupCtx, commentIDs, userID)

		return nil
	})
	group.Go(func() error {
		countMap, countMissIDs = s.likeCache.Counts(groupCtx, commentIDs)

		return nil
	})
	group.Go(func() error {
		replyCountMap = s.likeCache.ReplyCounts(groupCtx, commentIDs)

		return nil
	})
	_ = group.Wait()

	if userID > 0 {
		// Redis 失败（nil）全量回源；部分 miss 仅回源 miss 评论
		missIDs := likedMissIDs
		if likedMap == nil {
			likedMap = map[uint]bool{}
			missIDs = commentIDs
		}

		if len(missIDs) > 0 {
			// 回源失败按「未点赞」处理，不影响列表返回。
			dbLiked, _ := s.likes.LikedCommentIDs(ctx, userID, missIDs)
			for id, liked := range dbLiked {
				if liked {
					likedMap[id] = true
				}
			}
			// miss 异步全量重建 set+count（不阻塞列表）
			rebuildComment = append(rebuildComment, missIDs...)
		}
	}

	if likedMap == nil {
		likedMap = map[uint]bool{}
	}

	if countMap == nil {
		countMap = map[uint]int64{}
		rebuildComment = append(rebuildComment, commentIDs...)
	} else if len(countMissIDs) > 0 {
		rebuildComment = append(rebuildComment, countMissIDs...)
	}

	if len(rebuildComment) > 0 {
		s.cache.scheduleLikeCacheRebuild(ctx, rebuildComment...)
	}

	if replyCountMap == nil {
		replyCountMap = map[uint]int64{}
	}

	items := make([]CommentItem, len(rows))

	for i, r := range rows {
		likeCount := r.LikeCount
		if v, ok := countMap[r.ID]; ok {
			likeCount = v
		}

		replyCount := r.ReplyCount
		if v, ok := replyCountMap[r.ID]; ok {
			replyCount = v
		}

		items[i] = CommentItem{
			ID:            r.ID,
			Content:       r.Content,
			UserID:        r.UserID,
			Username:      r.Username,
			Avatar:        r.Avatar,
			VideoID:       r.VideoID,
			DynamicID:     r.DynamicID,
			VideoTitle:    r.VideoTitle,
			VideoCover:    r.VideoCover,
			ParentID:      r.ParentID,
			RootID:        r.RootID,
			AtUserIDs:     r.AtUserIDs,
			Pictures:      r.Pictures,
			LikeCount:     likeCount,
			ReplyCount:    replyCount,
			IsPinned:      r.IsPinned,
			PinnedAt:      r.PinnedAt,
			CommenterAddr: r.CommenterAddr,
			CreatedAt:     r.CreatedAt,
			IsLiked:       likedMap[r.ID],
			ReplyTo:       r.ReplyTo,
		}
	}

	return items
}

// QueryService 负责评论查询与列表投影。
type QueryService struct {
	likes     commentdomain.LikeRepository
	query     Query
	related   RelatedQuery
	likeCache LikeCache
	cache     *LikeCacheCoordinator
	pages     commentdomain.PagePolicy
	creators  commentdomain.CreatorPolicy
}

func NewQueryService(likes commentdomain.LikeRepository, query Query, related RelatedQuery, likeCache LikeCache, cache *LikeCacheCoordinator) *QueryService {
	return &QueryService{
		likes: likes, query: query, related: related, likeCache: likeCache, cache: cache,
		pages: commentdomain.PagePolicy{}, creators: commentdomain.CreatorPolicy{},
	}
}
