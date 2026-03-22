package comment

import (
	"context"

	commentdomain "f_ccg_video_back_ddd/internal/domain/comment"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

// AdminDelete 管理员批量删除评论（硬删除），自动处理子评论/点赞记录/视频评论计数缓存。
//
// 返回实际删除的评论数（含被连带删除的回复）。
func (s *DeleteService) AdminDelete(ctx context.Context, commentIDs []uint) (int64, error) {
	if err := s.deletes.RequireIDs(commentIDs); err != nil {
		return 0, err
	}

	comments, err := s.comments.BriefByIDs(ctx, commentIDs)
	if err != nil {
		return 0, err
	}

	if len(comments) == 0 {
		return 0, nil
	}

	rootIDs := s.deletes.RootIDs(comments)

	var children []commentdomain.Comment

	if len(rootIDs) > 0 {
		children, err = s.comments.ChildrenBrief(ctx, rootIDs)
		if err != nil {
			return 0, err
		}
	}

	plan := s.deletes.Removal(comments, children)
	if len(plan.IDs) == 0 {
		return 0, nil
	}

	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		// 更新根评论 reply_count（仅删除回复且根评论未删除时）
		for rootID, dec := range plan.ReplyDeltas {
			if dec <= 0 {
				continue
			}

			if err := s.comments.DecrReplyCountBy(txCtx, rootID, dec); err != nil {
				return err
			}
		}

		if err := s.likes.DeleteByCommentIDs(txCtx, plan.IDs); err != nil {
			return err
		}

		return s.comments.DeleteByIDs(txCtx, plan.IDs)
	}); err != nil {
		return 0, err
	}

	// 更新视频统计缓存（最佳努力）
	for videoID, dec := range plan.VideoDeltas {
		if dec <= 0 {
			continue
		}

		s.videos.UpdateCommentCount(ctx, videoID, -dec)
		s.hotScore.RecordHotScoreForComment(ctx, videoID, -dec)
	}

	// 清理点赞缓存是后台最佳努力收尾，脱离请求取消但保留短超时。
	cleanCtx, cancel := ctxutil.WithDetachedTimeout(ctx, ctxutil.ExternalShortTimeout)
	defer cancel()

	for _, id := range plan.IDs {
		s.likeCache.Drop(cleanCtx, id)
	}

	s.logger.Info("admin delete comments ok", zap.Int("count", len(plan.IDs)))

	return int64(len(plan.IDs)), nil
}
