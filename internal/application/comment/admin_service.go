package comment

import (
	"context"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"f_ccg_video_back_ddd/pkg/logctx"
	"strings"

	domain "f_ccg_video_back_ddd/internal/domain/comment"

	"go.uber.org/zap"
)

// AdminList 管理端评论列表（原 service/comment_service/admin_service.go AdminList）。
//
// 管理端不需要 isLiked（视作未登录），但点赞数仍以 Redis 缓存为主。
func (s *Service) AdminList(ctx context.Context, params AdminListParams) ([]Item, int64, error) {
	page, pageSize := domain.NormalizeAdminPaging(params.Page, params.PageSize)

	rows, total, err := s.query.AdminList(ctx, AdminListQuery{
		UserID:   params.UserID,
		Username: strings.TrimSpace(params.Username),
		Keyword:  strings.TrimSpace(params.Keyword),
		Sort:     params.Sort,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, 0, err
	}

	return s.buildCommentItems(ctx, rows, 0), total, nil
}

// AdminDelete 管理员批量删除评论（硬删除），自动处理子评论/点赞记录/视频评论计数缓存。
//
// 返回实际删除的评论数（含被连带删除的回复）。
func (s *Service) AdminDelete(ctx context.Context, commentIDs []uint) (int64, error) {
	if len(commentIDs) == 0 {
		return 0, domain.ErrCommentIDsRequired
	}

	comments, err := s.comments.BriefByIDs(ctx, commentIDs)
	if err != nil {
		return 0, err
	}

	if len(comments) == 0 {
		return 0, nil
	}

	rootIDs := make([]uint, 0, len(comments))
	rootIDSet := make(map[uint]struct{})
	replyRootDelta := make(map[uint]int64)

	for _, c := range comments {
		if c.ID == 0 {
			continue
		}

		if c.RootID == nil {
			rootIDs = append(rootIDs, c.ID)
			rootIDSet[c.ID] = struct{}{}

			continue
		}
		// 删除回复时，如果根评论不在本次删除列表，需要扣减 reply_count
		if _, ok := rootIDSet[*c.RootID]; ok {
			continue
		}

		replyRootDelta[*c.RootID]++
	}

	// 如果删除了根评论，需要把子评论一并删除
	var children []domain.Comment

	if len(rootIDs) > 0 {
		children, err = s.comments.ChildrenBrief(ctx, rootIDs)
		if err != nil {
			return 0, err
		}
	}

	return finishServiceAdminDelete(ctx, s, comments, rootIDSet, replyRootDelta, children)
}

func serviceAdminDeleteTransaction(txCtx context.Context, s *Service, rootIDSet map[uint]struct{}, replyRootDelta map[uint]int64, idsToDelete []uint) error {
	// 更新根评论 reply_count（仅删除回复且根评论未删除时）
	for rootID, dec := range replyRootDelta {
		if dec <= 0 {
			continue
		}

		if _, ok := rootIDSet[rootID]; ok {
			continue
		}

		if err := s.comments.DecrReplyCountBy(txCtx, rootID, dec); err != nil {
			return err
		}
	}

	if err := s.likes.DeleteByCommentIDs(txCtx, idsToDelete); err != nil {
		return err
	}

	return s.comments.DeleteByIDs(txCtx, idsToDelete)
}

func finishServiceAdminDelete(ctx context.Context, s *Service, comments []domain.Comment, rootIDSet map[uint]struct{}, replyRootDelta map[uint]int64, children []domain.Comment) (int64, error) {
	videoDelta := make(map[uint]int64)
	idsToDeleteSet := make(map[uint]struct{}, len(comments)+len(children))

	var idsToDelete []uint

	push := func(c domain.Comment) {
		if c.ID == 0 {
			return
		}

		if _, ok := idsToDeleteSet[c.ID]; ok {
			return
		}

		idsToDeleteSet[c.ID] = struct{}{}

		idsToDelete = append(idsToDelete, c.ID)
		if c.VideoID > 0 {
			videoDelta[c.VideoID]++
		}
	}
	for _, c := range comments {
		push(c)
	}

	for _, c := range children {
		push(c)
	}

	if len(idsToDelete) == 0 {
		return 0, nil
	}

	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		return serviceAdminDeleteTransaction(txCtx, s, rootIDSet, replyRootDelta, idsToDelete)
	}); err != nil {
		return 0, err
	}

	// 更新视频统计缓存（最佳努力）
	for videoID, dec := range videoDelta {
		if dec <= 0 {
			continue
		}

		s.videos.UpdateCommentCount(ctx, videoID, -dec)
		s.hotScore.RecordHotScoreForComment(ctx, videoID, -dec)
	}

	// 清理点赞缓存是后台最佳努力收尾，脱离请求取消但保留短超时。
	cleanCtx, cancel := ctxutil.WithDetachedTimeout(ctx, ctxutil.ExternalShortTimeout)
	defer cancel()

	for _, id := range idsToDelete {
		s.likeCache.Drop(cleanCtx, id)
	}

	logctx.With(ctx, s.logger).Info("admin delete comments ok", zap.String("event", "comment.admin_service.admin_delete.diagnostic"), zap.String("operation", "comment.admin_service.admin_delete"), zap.Int("count", len(idsToDelete)))

	return int64(len(idsToDelete)), nil
}
