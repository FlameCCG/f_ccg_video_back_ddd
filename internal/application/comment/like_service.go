package comment

import (
	"context"
	"errors"

	commentdomain "f_ccg_video_back_ddd/internal/domain/comment"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"

	"go.uber.org/zap"
)

// ToggleLike 点赞/取消点赞评论。
//
// 优先走 Redis Lua 原子切换（成功即返回，落库由回写队列异步完成）；
// 缓存不可用或失败时回退 DB 事务，保证成功路径的数据持久化。
func (s *LikeService) ToggleLike(ctx context.Context, userID, commentID uint) (int64, bool, error) {
	if err := s.login.Require(userID); err != nil {
		return 0, false, err
	}

	// 校验评论存在（并拦截无效 ID）
	target, err := s.comments.FindLikeTarget(ctx, commentID)
	if err != nil {
		return 0, false, err
	}

	if target == nil {
		return 0, false, commentdomain.ErrCommentNotFound
	}

	if s.likeCache.Available() {
		// Set+Count 不完整则同步从 DB 重建（无 seeded 标记）
		if err := s.cache.ensureLikeCacheReady(ctx, commentID); err != nil {
			s.logger.Warn("ensure comment like cache ready failed, fallback to db",
				zap.Uint("commentId", commentID), zap.Error(err))
		} else {
			likeCount, isLiked, toggleErr := s.likeCache.Toggle(ctx, commentID, userID)
			if toggleErr == nil {
				if isLiked {
					s.notifications.scheduleLikeNotification(ctx, userID, commentID)
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
			s.notifications.scheduleLikeNotification(ctx, userID, commentID)
		}
	}

	return likeCount, isLiked, dbErr
}

// toggleLikeWithDB DB 回退路径：读当前点赞记录，落库切换后重读点赞数。
func (s *LikeService) toggleLikeWithDB(ctx context.Context, userID, commentID uint) (int64, bool, error) {
	var count int64
	liked := false
	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		_, err := s.likes.Create(txCtx, &commentdomain.Like{CommentID: commentID, UserID: userID})
		if err == nil {
			liked = true
			if err := s.comments.IncrLikeCount(txCtx, commentID); err != nil {
				return err
			}
		} else if errors.Is(err, commentdomain.ErrLikeDuplicated) {
			existing, err := s.likes.Find(txCtx, commentID, userID)
			if err != nil {
				return err
			}
			if existing != nil {
				if err := s.likes.DeleteByID(txCtx, existing.ID); err != nil {
					return err
				}
				if err := s.comments.DecrLikeCount(txCtx, commentID); err != nil {
					return err
				}
			}
		} else {
			return err
		}
		var countErr error
		count, countErr = s.comments.LikeCountByID(txCtx, commentID)
		return countErr
	})
	return count, liked, err
}

// LikeService 负责点赞切换与数据库降级。
type LikeService struct {
	txm           tx.Manager
	comments      commentdomain.Repository
	likes         commentdomain.LikeRepository
	likeCache     LikeCache
	logger        *zap.Logger
	cache         *LikeCacheCoordinator
	notifications *NotificationDispatcher
	login         commentdomain.LoginPolicy
}

func NewLikeService(txm tx.Manager, comments commentdomain.Repository, likes commentdomain.LikeRepository, likeCache LikeCache, logger *zap.Logger, cache *LikeCacheCoordinator, notifications *NotificationDispatcher) *LikeService {
	return &LikeService{
		txm: txm, comments: comments, likes: likes, likeCache: likeCache, logger: logger,
		cache: cache, notifications: notifications, login: commentdomain.LoginPolicy{},
	}
}
