package comment

import (
	"context"
	"fmt"
	"strings"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/comment"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// likeCacheDBTimeout 点赞缓存重建时读库 + 回写缓存的超时（与原实现的 2s 一致）。
const likeCacheDBTimeout = 2 * time.Second

// 通知文案（与原 service/comment_service 逐字一致）。
const (
	noticeTitleVideoComment   = "有人评论了你的视频"
	noticeTitleDynamicComment = "有人评论了你的动态"
	noticeTitleReply          = "有人回复了你"
	noticeTitleAtInComment    = "在评论中@了我"
	noticeTitleAtInDynamic    = "在动态评论中@了我"
	noticeTitleLikeComment    = "有人点赞了你的评论"
)

// Service 评论用例服务（原 service/comment_service.CommentService）。
//
// 原实现是空结构体 + 直读 global.DB / global.RedisClient，并直接 import
// video_service 与 notification_service；新架构改为构造注入 + 本地端口。
type Service struct {
	txm       tx.Manager
	comments  domain.Repository
	likes     domain.LikeRepository
	query     Query
	related   RelatedQuery
	likeCache LikeCache
	notifier  Notifier
	videos    VideoCommentCounter
	hotScore  VideoHotScoreRecorder
	tasks     *ctxutil.DetachedTaskGroup
	logger    *zap.Logger
}

// NewService 构造评论用例服务。
func NewService(
	txm tx.Manager,
	comments domain.Repository,
	likes domain.LikeRepository,
	query Query,
	related RelatedQuery,
	likeCache LikeCache,
	notifier Notifier,
	videos VideoCommentCounter,
	hotScore VideoHotScoreRecorder,
	tasks *ctxutil.DetachedTaskGroup,
	logger *zap.Logger,
) *Service {
	return &Service{
		txm:       txm,
		comments:  comments,
		likes:     likes,
		query:     query,
		related:   related,
		likeCache: likeCache,
		notifier:  notifier,
		videos:    videos,
		hotScore:  hotScore,
		tasks:     tasks,
		logger:    logger,
	}
}

// Create 发布评论。
//
// 流程与原 CommentService.Create 一致：并发预取（评论者 / 评论对象 / @ 名单）→
// 回复时校验父评论并推导 rootID → 落库（回复与根评论回复数同事务）→
// 同步更新视频评论数 → 异步热度打点与通知。
func (s *Service) Create(ctx context.Context, cmd CreateCommand) (*CommentItem, error) {
	draft, commentType, err := domain.NormalizeDraft(domain.Draft{
		UserID:        cmd.UserID,
		VideoID:       cmd.VideoID,
		DynamicID:     cmd.DynamicID,
		Content:       cmd.Content,
		ParentID:      cmd.ParentID,
		AtUserIDs:     cmd.AtUserIDs,
		Pictures:      cmd.Pictures,
		CommenterIP:   cmd.IP,
		CommenterAddr: cmd.Addr,
	})
	if err != nil {
		return nil, err
	}

	var (
		video      *VideoBrief
		dynamic    *DynamicBrief
		actor      = UserProfile{ID: draft.UserID}
		filteredAt []uint
	)

	candidates := domain.NormalizeAtUserIDs(draft.UserID, draft.AtUserIDs)

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		profile, err := s.related.UserProfile(groupCtx, draft.UserID)
		if err != nil {
			return err
		}
		// 用户记录缺失时保留最小 actor，避免因缓存过期账号阻塞发评
		if profile != nil {
			actor = *profile
		}

		return nil
	})

	if draft.VideoID > 0 {
		group.Go(func() error {
			brief, err := s.related.PublishedVideoBrief(groupCtx, draft.VideoID)
			if err != nil {
				return err
			}

			if brief == nil {
				return domain.ErrVideoNotPublished
			}

			video = brief

			return nil
		})
	} else {
		group.Go(func() error {
			brief, err := s.related.DynamicBrief(groupCtx, draft.DynamicID)
			if err != nil {
				return err
			}

			if brief == nil {
				return domain.ErrDynamicNotFound
			}

			dynamic = brief

			return nil
		})
	}

	if len(candidates) > 0 {
		group.Go(func() error {
			filteredAt = s.related.AllowedAtUserIDs(groupCtx, draft.UserID, candidates)

			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}

	draft.AtUserIDs = filteredAt

	// 如果是回复，检查父评论是否存在并自动计算 rootID
	var (
		replyTo *ReplyInfo
		rootID  *uint
	)

	if domain.HasParent(draft.ParentID) {
		parent, err := s.query.ParentBrief(ctx, *draft.ParentID, draft.VideoID, draft.DynamicID)
		if err != nil {
			return nil, err
		}

		if parent == nil {
			return nil, domain.ErrParentNotFound
		}

		replyTo = &ReplyInfo{
			UserID:   parent.UserID,
			Username: parent.Username,
		}
		rootID = domain.ResolveRootID(*draft.ParentID, *parent)
	}

	entity := domain.NewComment(draft, commentType, rootID)

	if rootID != nil && *rootID > 0 {
		if err := s.txm.Do(ctx, func(txCtx context.Context) error {
			if err := s.comments.Create(txCtx, entity); err != nil {
				return err
			}

			return s.comments.IncrReplyCount(txCtx, *rootID)
		}); err != nil {
			return nil, err
		}
	} else if err := s.comments.Create(ctx, entity); err != nil {
		return nil, err
	}

	// 发送评论通知（作者/被回复者）
	userSnapshot := actor
	commentSnapshot := *entity

	if draft.VideoID > 0 {
		// 评论数同步更新，避免进程崩溃导致计数丢失；热度可异步
		s.videos.UpdateCommentCount(ctx, draft.VideoID, 1)

		videoIDSnapshot := draft.VideoID
		s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
			s.hotScore.RecordHotScoreForComment(asyncCtx, videoIDSnapshot, 1)
		})
	}

	switch {
	case video != nil:
		videoSnapshot := *video

		s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
			s.sendCommentCreateNotifications(asyncCtx, &userSnapshot, &videoSnapshot, &commentSnapshot, replyTo)
		})
	case dynamic != nil:
		dynamicSnapshot := *dynamic

		s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
			s.sendDynamicCommentCreateNotifications(asyncCtx, &userSnapshot, &dynamicSnapshot, &commentSnapshot, replyTo)
		})
	}

	// 发送 @ 通知
	if len(draft.AtUserIDs) > 0 {
		atUserIDs := draft.AtUserIDs
		content := draft.Content

		switch {
		case video != nil:
			videoSnapshot := *video

			s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
				s.sendAtNotifications(asyncCtx, &userSnapshot, &videoSnapshot, commentSnapshot.ID, content, atUserIDs)
			})
		case dynamic != nil:
			dynamicIDSnapshot := dynamic.ID

			s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
				s.sendAtNotificationsForDynamic(asyncCtx, &userSnapshot, dynamicIDSnapshot, commentSnapshot.ID, content, atUserIDs)
			})
		}
	}

	return &CommentItem{
		ID:            entity.ID,
		Content:       entity.Content,
		UserID:        entity.UserID,
		Username:      actor.Username,
		Avatar:        actor.Avatar,
		VideoID:       entity.VideoID,
		DynamicID:     entity.DynamicID,
		ParentID:      entity.ParentID,
		RootID:        entity.RootID,
		AtUserIDs:     entity.AtUserIDs,
		Pictures:      entity.Pictures,
		LikeCount:     0,
		ReplyCount:    0,
		CommenterAddr: entity.CommenterAddr,
		CreatedAt:     entity.CreatedAt,
		IsLiked:       false,
		ReplyTo:       replyTo,
	}, nil
}

// Delete 删除评论（评论作者或作品作者可删）。
//
// 根评论连带删除其全部回复与相关点赞记录；子评论则回扣根评论的回复数。
func (s *Service) Delete(ctx context.Context, userID, commentID uint) error {
	target, err := s.comments.FindByID(ctx, commentID)
	if err != nil {
		return err
	}

	if target == nil {
		return domain.ErrCommentNotFound
	}

	if err := s.authorizeDelete(ctx, target, userID); err != nil {
		return err
	}

	// 计算要删除的评论数（包括子评论）
	var deleteCount int64 = 1

	relatedIDs := []uint{commentID}

	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		if target.IsRoot() {
			// 是根评论，统计所有子评论
			childIDs, err := s.comments.ChildIDs(txCtx, commentID)
			if err != nil {
				return err
			}

			if len(childIDs) > 0 {
				relatedIDs = append(relatedIDs, childIDs...)
			}

			deleteCount = int64(len(childIDs)) + 1 // 加上根评论本身
			// 删除所有子评论
			if err := s.comments.DeleteByRootID(txCtx, commentID); err != nil {
				return err
			}
		} else if err := s.comments.DecrReplyCount(txCtx, *target.RootID); err != nil {
			// 是子评论，更新根评论的回复数
			return err
		}

		// 删除评论本身
		if err := s.comments.DeleteByID(txCtx, commentID); err != nil {
			return err
		}

		// 删除相关点赞记录
		return s.likes.DeleteByCommentIDs(txCtx, relatedIDs)
	}); err != nil {
		return err
	}

	// 更新视频统计缓存
	if target.VideoID > 0 {
		s.videos.UpdateCommentCount(ctx, target.VideoID, -deleteCount)
		s.hotScore.RecordHotScoreForComment(ctx, target.VideoID, -deleteCount)
	}

	for _, id := range relatedIDs {
		s.likeCache.Drop(ctx, id)
	}

	return nil
}

// authorizeDelete 删除鉴权：评论作者直接放行，否则要求是作品作者。
func (s *Service) authorizeDelete(ctx context.Context, target *domain.Comment, userID uint) error {
	switch domain.DecideDelete(target, userID) {
	case domain.DeleteAllowed:
		return nil
	case domain.DeleteNeedsVideoAuthorCheck:
		isAuthor, err := s.isVideoAuthor(ctx, userID, target.VideoID)
		if err != nil {
			return err
		}

		if !isAuthor {
			return domain.ErrDeleteForbidden
		}

		return nil
	case domain.DeleteNeedsDynamicAuthorCheck:
		isAuthor, err := s.isDynamicAuthor(ctx, userID, target.DynamicID)
		if err != nil {
			return err
		}

		if !isAuthor {
			return domain.ErrDeleteForbidden
		}

		return nil
	default:
		return domain.ErrDeleteForbidden
	}
}

// Pin 置顶/取消置顶评论（仅作品作者）。
func (s *Service) Pin(ctx context.Context, userID, commentID uint, pinned bool) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	if commentID == 0 {
		return domain.ErrCommentIDRequired
	}

	target, err := s.comments.FindPinTarget(ctx, commentID)
	if err != nil {
		return err
	}

	if target == nil {
		return domain.ErrCommentNotFound
	}

	if err := domain.ValidatePinTarget(target); err != nil {
		return err
	}

	var isAuthor bool

	if target.VideoID > 0 {
		isAuthor, err = s.isVideoAuthor(ctx, userID, target.VideoID)
	} else {
		isAuthor, err = s.isDynamicAuthor(ctx, userID, target.DynamicID)
	}

	if err != nil {
		return err
	}

	if !isAuthor {
		return domain.ErrPinForbidden
	}

	now := time.Now()

	return s.txm.Do(ctx, func(txCtx context.Context) error {
		if !pinned {
			return s.comments.SetPinned(txCtx, target.ID, false, nil)
		}
		// 同一作品下只保留一条置顶评论
		if target.VideoID > 0 {
			if err := s.comments.UnpinByVideo(txCtx, target.VideoID); err != nil {
				return err
			}
		} else if err := s.comments.UnpinByDynamic(txCtx, target.DynamicID); err != nil {
			return err
		}

		return s.comments.SetPinned(txCtx, target.ID, true, &now)
	})
}

// List 获取作品的根评论列表（分页）。
func (s *Service) List(ctx context.Context, params ListParams) ([]CommentItem, int64, error) {
	if _, err := domain.ResolveTargetType(params.VideoID, params.DynamicID); err != nil {
		return nil, 0, err
	}

	page, pageSize := domain.NormalizePaging(params.Page, params.PageSize)

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
func (s *Service) Replies(ctx context.Context, params RepliesParams) ([]CommentItem, int64, error) {
	page, pageSize := domain.NormalizePaging(params.Page, params.PageSize)

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
func (s *Service) ListByAuthor(ctx context.Context, params CreatorListParams) ([]CommentItem, int64, error) {
	if params.AuthorID == 0 {
		return nil, 0, domain.ErrAuthorIDRequired
	}

	page, pageSize := domain.NormalizePaging(params.Page, params.PageSize)

	username := s.related.UsernameByID(ctx, params.AuthorID)
	if username == "" {
		return nil, 0, domain.ErrAuthorNotFound
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
func (s *Service) GetCount(ctx context.Context, videoID uint) (int64, error) {
	return s.query.CountByVideo(ctx, videoID)
}

// ToggleLike 点赞/取消点赞评论。
//
// 优先走 Redis Lua 原子切换（成功即返回，落库由回写队列异步完成）；
// 缓存不可用或失败时回退 DB 事务，保证成功路径的数据持久化。
func (s *Service) ToggleLike(ctx context.Context, userID, commentID uint) (int64, bool, error) {
	if userID == 0 {
		return 0, false, domain.ErrUserNotLogin
	}

	// 校验评论存在（并拦截无效 ID）
	target, err := s.comments.FindLikeTarget(ctx, commentID)
	if err != nil {
		return 0, false, err
	}

	if target == nil {
		return 0, false, domain.ErrCommentNotFound
	}

	if s.likeCache.Available() {
		// Set+Count 不完整则同步从 DB 重建（无 seeded 标记）
		if err := s.ensureLikeCacheReady(ctx, commentID); err != nil {
			s.logger.Warn("ensure comment like cache ready failed, fallback to db",
				zap.Uint("commentId", commentID), zap.Error(err))
		} else {
			likeCount, isLiked, toggleErr := s.likeCache.Toggle(ctx, commentID, userID)
			if toggleErr == nil {
				if isLiked {
					s.scheduleLikeNotification(ctx, userID, commentID)
				}

				return likeCount, isLiked, nil
			}

			s.logger.Warn("toggle like via redis failed, fallback to db",
				zap.Uint("commentId", commentID), zap.Error(toggleErr))
		}
	}

	likeCount, isLiked, dbErr := s.toggleLikeWithDB(ctx, userID, commentID)
	if dbErr == nil {
		// 禁止用滞后 DB like_count 绝对 SET 覆盖 Redis；清缓存待下次重建
		s.likeCache.Drop(ctx, commentID)

		if isLiked {
			s.scheduleLikeNotification(ctx, userID, commentID)
		}
	}

	return likeCount, isLiked, dbErr
}

// toggleLikeWithDB DB 回退路径：读当前点赞记录，落库切换后重读点赞数。
func (s *Service) toggleLikeWithDB(ctx context.Context, userID, commentID uint) (int64, bool, error) {
	existing, err := s.likes.Find(ctx, commentID, userID)
	if err != nil {
		return 0, false, err
	}

	isLiked := existing == nil

	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		if existing == nil {
			// 点赞
			like := &domain.Like{CommentID: commentID, UserID: userID}
			if err := s.likes.Create(txCtx, like); err != nil {
				return err
			}

			return s.comments.IncrLikeCount(txCtx, commentID)
		}
		// 取消点赞
		if err := s.likes.DeleteByID(txCtx, existing.ID); err != nil {
			return err
		}

		return s.comments.DecrLikeCount(txCtx, commentID)
	}); err != nil {
		return 0, false, err
	}

	// 与原实现一致：提交后重读点赞数并忽略读取错误（失败时返回 0）。
	likeCount, _ := s.comments.LikeCountByID(ctx, commentID)

	return likeCount, isLiked, nil
}

// ensureLikeCacheReady 点赞热路径：缓存不完整则查 DB。
//   - DB 有赞：全量灌 set+count
//   - DB=0：只清半残 key，不先置 count=0；由本次 Lua 用当前操作初始化并入队
func (s *Service) ensureLikeCacheReady(ctx context.Context, commentID uint) error {
	ok, err := s.likeCache.Complete(ctx, commentID)
	if err != nil {
		return err
	}

	if ok {
		return nil
	}

	cacheCtx, cancel := context.WithTimeout(ctx, likeCacheDBTimeout)
	defer cancel()

	userIDs, err := s.likes.UserIDsByComment(cacheCtx, commentID)
	if err != nil {
		return err
	}

	if len(userIDs) > 0 {
		return s.likeCache.Write(cacheCtx, commentID, userIDs)
	}

	// DB=0：不写 count=0，清掉半残后交给 toggle Lua（GET miss 当 0 + 当前 SADD/入队）
	return s.likeCache.Clear(cacheCtx, commentID)
}

// rebuildLikeCache 以 comment_likes 为准全量重建 set+count（同 TTL）。
// 读路径 / 异步重建使用：DB=0 时会写入 count=0，避免列表反复 miss。
func (s *Service) rebuildLikeCache(ctx context.Context, commentID uint) error {
	if !s.likeCache.Available() || commentID == 0 {
		return domain.ErrLikeCacheUnavailable
	}

	cacheCtx, cancel := context.WithTimeout(ctx, likeCacheDBTimeout)
	defer cancel()

	userIDs, err := s.likes.UserIDsByComment(cacheCtx, commentID)
	if err != nil {
		return err
	}

	return s.likeCache.Write(cacheCtx, commentID, userIDs)
}

// scheduleLikeCacheRebuild 读路径 miss 时异步全量重建，不阻塞响应。
func (s *Service) scheduleLikeCacheRebuild(parent context.Context, commentIDs ...uint) {
	if !s.likeCache.Available() || len(commentIDs) == 0 {
		return
	}

	// 去重
	seen := make(map[uint]struct{}, len(commentIDs))
	ids := make([]uint, 0, len(commentIDs))

	for _, id := range commentIDs {
		if id == 0 {
			continue
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return
	}

	s.tasks.Go(parent, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
		for _, id := range ids {
			if err := s.rebuildLikeCache(asyncCtx, id); err != nil {
				s.logger.Warn("async rebuild comment like cache failed", zap.Uint("commentId", id), zap.Error(err))
			}
		}
	})
}

// buildCommentItems 构建评论返回结构：叠加点赞数/回复数缓存与当前用户点赞状态。
func (s *Service) buildCommentItems(ctx context.Context, rows []CommentRow, userID uint) []CommentItem {
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
			// 与原实现一致：回源失败按「未点赞」处理，不影响列表返回。
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
		s.scheduleLikeCacheRebuild(ctx, rebuildComment...)
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

// isVideoAuthor 判断用户是否为视频作者（用于删除/置顶评论的授权）。
func (s *Service) isVideoAuthor(ctx context.Context, userID, videoID uint) (bool, error) {
	ref, err := s.related.VideoAuthorRef(ctx, videoID)
	if err != nil {
		return false, err
	}

	if ref == nil {
		return false, domain.ErrRecordNotFound
	}

	return ref.AuthorUserID == userID, nil
}

// isDynamicAuthor 判断用户是否为动态作者。
func (s *Service) isDynamicAuthor(ctx context.Context, userID, dynamicID uint) (bool, error) {
	brief, err := s.related.DynamicBrief(ctx, dynamicID)
	if err != nil {
		return false, err
	}

	if brief == nil {
		return false, domain.ErrRecordNotFound
	}

	return brief.UserID == userID, nil
}

// scheduleLikeNotification 异步发送评论点赞通知。
func (s *Service) scheduleLikeNotification(ctx context.Context, actionUserID, commentID uint) {
	s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
		s.sendCommentLikeNotification(asyncCtx, actionUserID, commentID)
	})
}

// sendCommentCreateNotifications 通知视频作者/被回复者。
func (s *Service) sendCommentCreateNotifications(
	ctx context.Context,
	actionUser *UserProfile,
	video *VideoBrief,
	c *domain.Comment,
	replyTo *ReplyInfo,
) {
	if actionUser == nil || video == nil || c == nil {
		return
	}

	receivers := newNoticeSet()
	// 视频作者
	if video.AuthorUsername != "" {
		author, err := s.related.UserProfileByUsername(ctx, video.AuthorUsername)
		if err == nil && author != nil && author.ID != actionUser.ID {
			receivers.put(author.ID, Notice{
				Type:             enum.NotificationApplyType,
				ReceiverID:       int(author.ID),
				ActionUserID:     actionUser.ID,
				ActionUserAvatar: actionUser.Avatar,
				ActionUserName:   actionUser.Username,
				Title:            noticeTitleVideoComment,
				Content:          c.Content,
				VideoID:          video.ID,
				VideoTitle:       video.Title,
				CommentID:        c.ID,
			})
		}
	}
	// 被回复者
	if replyTo != nil && replyTo.UserID > 0 && replyTo.UserID != actionUser.ID {
		receivers.put(replyTo.UserID, Notice{
			Type:             enum.NotificationApplyType,
			ReceiverID:       int(replyTo.UserID),
			ActionUserID:     actionUser.ID,
			ActionUserAvatar: actionUser.Avatar,
			ActionUserName:   actionUser.Username,
			Title:            noticeTitleReply,
			Content:          c.Content,
			VideoID:          video.ID,
			VideoTitle:       video.Title,
			CommentID:        c.ID,
		})
	}

	s.dispatchNotices(ctx, receivers.list(), "create comment notifications failed")
}

// sendDynamicCommentCreateNotifications 通知动态作者/被回复者。
func (s *Service) sendDynamicCommentCreateNotifications(
	ctx context.Context,
	actionUser *UserProfile,
	dynamic *DynamicBrief,
	c *domain.Comment,
	replyTo *ReplyInfo,
) {
	if actionUser == nil || dynamic == nil || c == nil {
		return
	}

	link := dynamicLink(dynamic.ID)

	receivers := newNoticeSet()
	// 动态作者
	if dynamic.UserID > 0 && dynamic.UserID != actionUser.ID {
		receivers.put(dynamic.UserID, Notice{
			Type:             enum.NotificationApplyType,
			ReceiverID:       int(dynamic.UserID),
			ActionUserID:     actionUser.ID,
			ActionUserAvatar: actionUser.Avatar,
			ActionUserName:   actionUser.Username,
			Title:            noticeTitleDynamicComment,
			Content:          c.Content,
			Link:             link,
			CommentID:        c.ID,
		})
	}
	// 被回复者
	if replyTo != nil && replyTo.UserID > 0 && replyTo.UserID != actionUser.ID {
		receivers.put(replyTo.UserID, Notice{
			Type:             enum.NotificationApplyType,
			ReceiverID:       int(replyTo.UserID),
			ActionUserID:     actionUser.ID,
			ActionUserAvatar: actionUser.Avatar,
			ActionUserName:   actionUser.Username,
			Title:            noticeTitleReply,
			Content:          c.Content,
			Link:             link,
			CommentID:        c.ID,
		})
	}

	s.dispatchNotices(ctx, receivers.list(), "create comment notifications failed")
}

// sendAtNotifications 视频评论中的 @ 通知。
func (s *Service) sendAtNotifications(
	ctx context.Context,
	actionUser *UserProfile,
	video *VideoBrief,
	commentID uint,
	content string,
	atUserIDs []uint,
) {
	if len(atUserIDs) == 0 || actionUser == nil || video == nil {
		return
	}
	// 去重
	seen := make(map[uint]struct{})

	notifications := make([]Notice, 0, len(atUserIDs))

	for _, uid := range atUserIDs {
		if uid == 0 {
			continue
		}

		if _, ok := seen[uid]; ok {
			continue
		}

		seen[uid] = struct{}{}
		notifications = append(notifications, Notice{
			Type:             enum.NotificationAtType,
			ReceiverID:       int(uid),
			ActionUserID:     actionUser.ID,
			ActionUserAvatar: actionUser.Avatar,
			ActionUserName:   actionUser.Username,
			Title:            noticeTitleAtInComment,
			Content:          content,
			VideoID:          video.ID,
			VideoTitle:       video.Title,
			CommentID:        commentID,
		})
	}

	s.dispatchNotices(ctx, notifications, "create @ notifications failed")
}

// sendAtNotificationsForDynamic 动态评论中的 @ 通知。
func (s *Service) sendAtNotificationsForDynamic(
	ctx context.Context,
	actionUser *UserProfile,
	dynamicID, commentID uint,
	content string,
	atUserIDs []uint,
) {
	if len(atUserIDs) == 0 || actionUser == nil || dynamicID == 0 {
		return
	}

	seen := make(map[uint]struct{})

	notifications := make([]Notice, 0, len(atUserIDs))

	for _, uid := range atUserIDs {
		if uid == 0 {
			continue
		}

		if _, ok := seen[uid]; ok {
			continue
		}

		seen[uid] = struct{}{}
		notifications = append(notifications, Notice{
			Type:             enum.NotificationAtType,
			ReceiverID:       int(uid),
			ActionUserID:     actionUser.ID,
			ActionUserAvatar: actionUser.Avatar,
			ActionUserName:   actionUser.Username,
			Title:            noticeTitleAtInDynamic,
			Content:          content,
			Link:             dynamicLink(dynamicID),
			CommentID:        commentID,
		})
	}

	s.dispatchNotices(ctx, notifications, "create @ notifications failed")
}

// sendCommentLikeNotification 点赞评论通知。
func (s *Service) sendCommentLikeNotification(ctx context.Context, actionUserID, commentID uint) {
	if actionUserID == 0 || commentID == 0 {
		return
	}

	actionUser, err := s.related.UserProfile(ctx, actionUserID)
	if err != nil || actionUser == nil {
		return
	}

	source, err := s.comments.FindNoticeSource(ctx, commentID)
	if err != nil || source == nil {
		return
	}

	if source.UserID == actionUserID {
		return
	}

	var (
		videoID    uint
		videoTitle string
	)

	if source.VideoID > 0 {
		// 与原实现一致：视频信息读取失败时留空，不阻断通知。
		if video, videoErr := s.related.VideoBrief(ctx, source.VideoID); videoErr == nil && video != nil {
			videoID = video.ID
			videoTitle = video.Title
		}
	}

	link := ""
	if source.DynamicID > 0 {
		link = dynamicLink(source.DynamicID)
	}

	notice := Notice{
		Type:             enum.NotificationLikeCommentType,
		ReceiverID:       int(source.UserID),
		ActionUserID:     actionUser.ID,
		ActionUserAvatar: actionUser.Avatar,
		ActionUserName:   actionUser.Username,
		Title:            noticeTitleLikeComment,
		Content:          source.Content,
		Link:             link,
		VideoID:          videoID,
		VideoTitle:       videoTitle,
		CommentID:        source.ID,
	}

	if err := s.notifier.CreateNotices(ctx, []Notice{notice}); err != nil {
		s.logger.Warn("create comment like notification failed", zap.Error(err))

		return
	}

	s.notifier.InvalidateCount(notice.ReceiverID)
}

// dispatchNotices 批量落库通知并按去重后的接收者失效角标缓存。
func (s *Service) dispatchNotices(ctx context.Context, notices []Notice, failMsg string) {
	if len(notices) == 0 {
		return
	}

	if err := s.notifier.CreateNotices(ctx, notices); err != nil {
		s.logger.Warn(failMsg, zap.Error(err))

		return
	}

	seen := make(map[int]struct{}, len(notices))

	for _, n := range notices {
		if _, ok := seen[n.ReceiverID]; ok {
			continue
		}

		seen[n.ReceiverID] = struct{}{}
		s.notifier.InvalidateCount(n.ReceiverID)
	}
}

// dynamicLink 动态详情跳转链接（格式与原实现一致）。
func dynamicLink(dynamicID uint) string {
	return fmt.Sprintf("/dynamic?dynamicId=%d", dynamicID)
}

// noticeSet 按接收者去重的通知集合，保持插入顺序、后写覆盖先写。
//
// 语义等于原实现的 map[uint]UserGlobalNotification（同一接收者只发一条，
// 「有人回复了你」覆盖「有人评论了你的视频/动态」），但落库顺序确定。
type noticeSet struct {
	order   []uint
	notices map[uint]Notice
}

func newNoticeSet() *noticeSet {
	return &noticeSet{notices: make(map[uint]Notice, 2)}
}

func (s *noticeSet) put(receiverID uint, notice Notice) {
	if _, ok := s.notices[receiverID]; !ok {
		s.order = append(s.order, receiverID)
	}

	s.notices[receiverID] = notice
}

func (s *noticeSet) list() []Notice {
	list := make([]Notice, 0, len(s.order))
	for _, id := range s.order {
		list = append(list, s.notices[id])
	}

	return list
}
