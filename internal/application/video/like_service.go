package video

import (
	"context"
	"errors"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

// likeActionAsyncGrace 点赞副作用（热度/经验）的 detached 超时。
const likeActionAsyncGrace = 500 * time.Millisecond

// videoLikeNotificationTitle 视频被点赞通知标题。
const videoLikeNotificationTitle = "有人点赞了你的视频"

// ToggleLike 点赞/取消点赞（唯一键翻转，stats 失败则回滚关系表）。
func (s *InteractionService) ToggleLike(ctx context.Context, videoID, userID uint) (*LikeResult, error) {
	if userID == 0 {
		return nil, domain.ErrUserNotLogin
	}

	if err := s.videos.EnsurePublished(ctx, videoID); err != nil {
		return nil, err
	}

	ctx = orTODO(ctx)

	// 事务内唯一键翻转关系表；返回 likes 增量（0 表示并发下已无变更）。
	delta, err := s.likes.Flip(ctx, videoID, userID)
	if err != nil {
		return nil, err
	}

	if delta == 0 {
		stats, loadErr := s.stats.Load(ctx, videoID)
		if loadErr != nil {
			return nil, loadErr
		}

		return &LikeResult{Likes: stats.Likes}, nil
	}

	stats, err := s.stats.ApplyDelta(ctx, videoID, domain.StatsDelta{Likes: delta})
	if err != nil {
		// Redis/DB stats 与关系表非同一事务：失败时回滚关系，避免 likes 漂移。
		if compErr := s.compensateLikeFlip(ctx, videoID, userID, delta); compErr != nil {
			zap.L().Error("compensate video like flip failed",
				zap.Uint("videoId", videoID),
				zap.Uint("userId", userID),
				zap.Int64("delta", delta),
				zap.Error(compErr),
				zap.NamedError("statsErr", err),
			)
		}

		return nil, err
	}

	s.recordLikeHotScoreAsync(ctx, videoID, delta)

	if delta > 0 {
		s.awardLikeExpAsync(ctx, userID)
		s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(notifyCtx context.Context) {
			s.sendVideoLikeNotification(notifyCtx, userID, videoID)
		})
	}

	return &LikeResult{Likes: stats.Likes}, nil
}

// compensateLikeFlip 在 stats 更新失败后回滚关系表变更。
// delta 为已应用到关系表的增量（+1 或 -1）。
func (s *InteractionService) compensateLikeFlip(ctx context.Context, videoID, userID uint, delta int64) error {
	switch {
	case delta > 0:
		// 刚点赞成功但 stats 失败 → 删掉关系
		_, err := s.likes.Delete(ctx, videoID, userID)

		return err
	case delta < 0:
		// 刚取消成功但 stats 失败 → 恢复关系
		err := s.likes.Create(ctx, videoID, userID)
		if errors.Is(err, domain.ErrDuplicateLike) {
			return nil
		}

		return err
	default:
		return nil
	}
}

func (s *InteractionService) recordLikeHotScoreAsync(parent context.Context, videoID uint, delta int64) {
	if videoID == 0 || delta == 0 {
		return
	}

	s.tasks.Go(parent, likeActionAsyncGrace, func(hotCtx context.Context) {
		s.stats.RecordHotScoreDelta(hotCtx, videoID, domain.HotScoreLikeWeight, delta)
	})
}

func (s *InteractionService) awardLikeExpAsync(parent context.Context, userID uint) {
	if userID == 0 {
		return
	}

	s.tasks.Go(parent, likeActionAsyncGrace, func(expCtx context.Context) {
		if err := s.exp.LikeExp(expCtx, userID); err != nil {
			zap.L().Warn("apply like exp failed", zap.Uint("userId", userID), zap.Error(err))
		}
	})
}

// sendVideoLikeNotification 视频被点赞时向作者发通知（作者是本人则跳过）。
func (s *InteractionService) sendVideoLikeNotification(ctx context.Context, actionUserID, videoID uint) {
	if actionUserID == 0 || videoID == 0 {
		return
	}

	actionUser, ok := s.notify.ActionUser(ctx, actionUserID)
	if !ok {
		return
	}

	authorID, videoTitle, ok := s.notify.VideoAuthorAndTitle(ctx, videoID)
	if !ok {
		return
	}

	if authorID == 0 || authorID == actionUserID {
		return
	}

	s.notifier.NotifyVideoLike(ctx, VideoNotice{
		ReceiverID:       authorID,
		ActionUserID:     actionUser.ID,
		ActionUserName:   actionUser.Username,
		ActionUserAvatar: actionUser.Avatar,
		Title:            videoLikeNotificationTitle,
		Content:          videoTitle,
		VideoID:          videoID,
		VideoTitle:       videoTitle,
	})
}
