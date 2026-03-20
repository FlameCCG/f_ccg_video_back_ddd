package video

import (
	"context"

	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	videoerrors "f_ccg_video_back_ddd/internal/domain/video/domainerr"
	videorepo "f_ccg_video_back_ddd/internal/domain/video/repository"

	"go.uber.org/zap"
)

// DeleteService 作者删稿用例服务。
type DeleteService struct {
	txm       tx.Manager
	videos    videorepo.VideoRepository
	comments  CommentPurger
	canceller TranscodeCanceller
	schedule  ScheduleStore
	authors   *AuthorService
	logger    *zap.Logger
}

// NewDeleteService 构造作者删稿用例服务。
func NewDeleteService(
	txm tx.Manager,
	videos videorepo.VideoRepository,
	comments CommentPurger,
	canceller TranscodeCanceller,
	schedule ScheduleStore,
	authors *AuthorService,
	logger *zap.Logger,
) *DeleteService {
	return &DeleteService{
		txm:       txm,
		videos:    videos,
		comments:  comments,
		canceller: canceller,
		schedule:  schedule,
		authors:   authors,
		logger:    logger,
	}
}

// DeleteVideo 将视频状态改为已删除（仅作者），并清理其评论与评论点赞。
func (s *DeleteService) DeleteVideo(ctx context.Context, videoID, userID uint) error {
	if videoID == 0 {
		return videoerrors.ErrVideoIDRequired
	}

	if userID == 0 {
		return videoerrors.ErrNotLoggedIn
	}

	var (
		deletedComments     int64
		deletedCommentLikes int64
		changed             bool
	)

	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		video, err := s.videos.FindByIDForUpdate(txCtx, videoID)
		if err != nil {
			return err
		}
		if video == nil {
			return videoerrors.ErrRecordNotFound
		}
		username, _ := s.videos.UsernameByUserID(txCtx, userID)
		changed, err = video.DeleteBy(username)
		if err != nil || !changed {
			return err
		}

		if updateErr := s.videos.UpdateStatus(txCtx, videoID, video.Status); updateErr != nil {
			return updateErr
		}

		comments, commentLikes, purgeErr := s.comments.PurgeByVideoIDs(txCtx, []uint{videoID})
		if purgeErr != nil {
			return purgeErr
		}

		deletedComments = comments
		deletedCommentLikes = commentLikes

		return s.schedule.Remove(txCtx, videoID)
	}); err != nil {
		return err
	}

	if !changed {
		return nil
	}

	if err := s.canceller.NotifyVideoDeleted(ctx, []uint{videoID}); err != nil {
		s.logger.Warn("notify transcode cancellation failed",
			zap.Uint("videoId", videoID),
			zap.Error(err),
		)
	}

	s.logger.Info("delete video ok",
		zap.Uint("videoId", videoID),
		zap.Int64("commentDeleted", deletedComments),
		zap.Int64("commentLikeDeleted", deletedCommentLikes),
	)
	s.authors.InvalidateVideoAuthorCache(ctx, videoID)

	return nil
}
