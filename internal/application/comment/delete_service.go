package comment

import (
	"context"

	commentdomain "f_ccg_video_back_ddd/internal/domain/comment"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"

	"go.uber.org/zap"
)

// Delete 删除评论（评论作者或作品作者可删）。
//
// 根评论连带删除其全部回复与相关点赞记录；子评论则回扣根评论的回复数。
func (s *DeleteService) Delete(ctx context.Context, userID, commentID uint) error {
	target, err := s.comments.FindByID(ctx, commentID)
	if err != nil {
		return err
	}

	if target == nil {
		return commentdomain.ErrCommentNotFound
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
		} else if err := s.comments.DecrReplyCount(txCtx, *target.RootID()); err != nil {
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
	if target.VideoID() > 0 {
		s.videos.UpdateCommentCount(ctx, target.VideoID(), -deleteCount)
		s.hotScore.RecordHotScoreForComment(ctx, target.VideoID(), -deleteCount)
	}

	for _, id := range relatedIDs {
		s.likeCache.Drop(ctx, id)
	}

	return nil
}

// authorizeDelete 删除鉴权：评论作者直接放行，否则要求是作品作者。
func (s *DeleteService) authorizeDelete(ctx context.Context, target *commentdomain.Comment, userID uint) error {
	switch target.DeletionDecision(userID) {
	case commentdomain.DeleteAllowed:
		return target.EnsureDeletionAuthorized(userID, false)
	case commentdomain.DeleteNeedsVideoAuthorCheck:
		isAuthor, err := (authorGate{related: s.related}).video(ctx, userID, target.VideoID())
		if err != nil {
			return err
		}

		return target.EnsureDeletionAuthorized(userID, isAuthor)
	case commentdomain.DeleteNeedsDynamicAuthorCheck:
		isAuthor, err := (authorGate{related: s.related}).dynamic(ctx, userID, target.DynamicID())
		if err != nil {
			return err
		}

		return target.EnsureDeletionAuthorized(userID, isAuthor)
	default:
		return target.EnsureDeletionAuthorized(userID, false)
	}
}

// DeleteService 负责删除与计数清理。
type DeleteService struct {
	txm       tx.Manager
	comments  commentdomain.Repository
	likes     commentdomain.LikeRepository
	related   RelatedQuery
	likeCache LikeCache
	videos    VideoCommentCounter
	hotScore  VideoHotScoreRecorder
	logger    *zap.Logger
	deletes   commentdomain.DeletePolicy
}

func NewDeleteService(txm tx.Manager, comments commentdomain.Repository, likes commentdomain.LikeRepository, related RelatedQuery, likeCache LikeCache, videos VideoCommentCounter, hotScore VideoHotScoreRecorder, logger *zap.Logger) *DeleteService {
	return &DeleteService{
		txm: txm, comments: comments, likes: likes, related: related, likeCache: likeCache,
		videos: videos, hotScore: hotScore, logger: logger, deletes: commentdomain.DeletePolicy{},
	}
}
