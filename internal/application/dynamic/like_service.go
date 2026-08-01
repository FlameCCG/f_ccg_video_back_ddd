package dynamic

import (
	"context"
	"errors"

	domain "f_ccg_video_back_ddd/internal/domain/dynamic"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

// LikeService 动态点赞用例服务（原 service/dynamic_service/like_service.go）。
type LikeService struct {
	txm      tx.Manager
	dynamics domain.DynamicRepository
	likes    domain.LikeRepository
	users    UserProfileQuery
	notifier DynamicLikeNotifier
	tasks    *ctxutil.DetachedTaskGroup
	logger   *zap.Logger
}

// NewLikeService 构造动态点赞用例服务。
func NewLikeService(
	txm tx.Manager,
	dynamics domain.DynamicRepository,
	likes domain.LikeRepository,
	users UserProfileQuery,
	notifier DynamicLikeNotifier,
	tasks *ctxutil.DetachedTaskGroup,
	logger *zap.Logger,
) *LikeService {
	return &LikeService{
		txm:      txm,
		dynamics: dynamics,
		likes:    likes,
		users:    users,
		notifier: notifier,
		tasks:    tasks,
		logger:   logger,
	}
}

// ToggleLike 切换动态点赞状态并返回最新点赞数。
//
// 幂等策略与原实现一致：先尝试插入点赞记录，唯一键冲突说明已点赞、转为取消；
// 计数用 SQL 表达式原子增减（取消时不低于 0），点赞成功后异步发通知。
func (s *LikeService) ToggleLike(ctx context.Context, userID, dynamicID uint) (*DynamicLikeResult, error) {
	if userID == 0 {
		return nil, domain.ErrLoginRequired
	}

	if dynamicID == 0 {
		return nil, domain.ErrDynamicIDRequired
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	exists, err := s.dynamics.ExistsByID(ctx, dynamicID)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, domain.ErrDynamicNotFound
	}

	liked := false

	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		createErr := s.likes.Create(txCtx, dynamicID, userID)
		if createErr == nil {
			liked = true

			return s.dynamics.IncrLikeCount(txCtx, dynamicID)
		}

		if errors.Is(createErr, domain.ErrLikeDuplicated) {
			affected, delErr := s.likes.Delete(txCtx, dynamicID, userID)
			if delErr != nil {
				return delErr
			}

			liked = false

			if affected == 0 {
				return nil
			}

			return s.dynamics.DecrLikeCount(txCtx, dynamicID)
		}

		return createErr
	}); err != nil {
		return nil, err
	}

	likeCount, err := s.dynamics.LikeCount(ctx, dynamicID)
	if err != nil {
		return nil, err
	}

	if liked {
		s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
			s.sendLikeNotification(asyncCtx, userID, dynamicID)
		})
	}

	return &DynamicLikeResult{LikeCount: likeCount, IsLiked: liked}, nil
}

// sendLikeNotification 向动态作者发送点赞通知（自赞与查库失败静默跳过）。
func (s *LikeService) sendLikeNotification(ctx context.Context, actionUserID, dynamicID uint) {
	if actionUserID == 0 || dynamicID == 0 {
		return
	}

	actionUser, err := s.users.ProfileByID(ctx, actionUserID)
	if err != nil || actionUser == nil {
		return
	}

	target, err := s.dynamics.FindNoticeTargetByID(ctx, dynamicID)
	if err != nil || target == nil {
		return
	}

	if target.UserID == 0 || target.UserID == actionUserID {
		return
	}

	if err := s.notifier.NotifyDynamicLike(
		ctx,
		target.UserID,
		target.ID,
		actionUser.ID,
		actionUser.Username,
		actionUser.Avatar,
		domain.LikeNoticeTitle,
		domain.LikeNoticeContent(target.Content),
	); err != nil {
		s.logger.Warn("create dynamic like notification failed", zap.Error(err))

		return
	}
}
