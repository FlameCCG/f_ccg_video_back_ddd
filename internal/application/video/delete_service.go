package video

import (
	"context"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	domain "f_ccg_video_back_ddd/internal/domain/video"

	"go.uber.org/zap"
)

// DeleteService 作者删稿用例服务（原 service/video_service/delete_service.go）。
type DeleteService struct {
	txm       tx.Manager
	videos    domain.VideoRepository
	comments  CommentPurger
	canceller TranscodeCanceller
	schedule  ScheduleStore
	authors   *AuthorService
}

// NewDeleteService 构造作者删稿用例服务。
func NewDeleteService(
	txm tx.Manager,
	videos domain.VideoRepository,
	comments CommentPurger,
	canceller TranscodeCanceller,
	schedule ScheduleStore,
	authors *AuthorService,
) *DeleteService {
	return &DeleteService{
		txm:       txm,
		videos:    videos,
		comments:  comments,
		canceller: canceller,
		schedule:  schedule,
		authors:   authors,
	}
}

// DeleteVideo 将视频状态改为已删除（仅作者），并清理其评论与评论点赞。
func (s *DeleteService) DeleteVideo(ctx context.Context, videoID, userID uint) error {
	if videoID == 0 {
		return domain.ErrVideoIDRequired
	}

	if userID == 0 {
		return domain.ErrNotLoggedIn
	}

	video, err := s.videos.FindByID(ctx, videoID)
	if err != nil {
		return err
	}

	if video == nil {
		return domain.ErrRecordNotFound
	}

	// 校验作者
	username, _ := s.videos.UsernameByUserID(ctx, userID)
	if username == "" || !video.OwnedBy(username) {
		return domain.ErrVideoDeleteForbidden
	}

	if video.IsDeleted() {
		return nil
	}

	var (
		deletedComments     int64
		deletedCommentLikes int64
	)

	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		if updateErr := s.videos.UpdateStatus(txCtx, videoID, enum.VideoStatusDeleted); updateErr != nil {
			return updateErr
		}

		comments, commentLikes, purgeErr := s.comments.PurgeByVideoIDs(txCtx, []uint{videoID})
		if purgeErr != nil {
			return purgeErr
		}

		deletedComments = comments
		deletedCommentLikes = commentLikes

		return nil
	}); err != nil {
		return err
	}

	if err := s.canceller.NotifyVideoDeleted(ctx, []uint{videoID}); err != nil {
		zap.L().Warn("notify transcode cancellation failed",
			zap.Uint("videoId", videoID),
			zap.Error(err),
		)
	}

	zap.L().Info("delete video ok",
		zap.Uint("videoId", videoID),
		zap.Int64("commentDeleted", deletedComments),
		zap.Int64("commentLikeDeleted", deletedCommentLikes),
	)
	_ = s.schedule.Remove(ctx, videoID)
	s.authors.InvalidateVideoAuthorCache(ctx, videoID)

	return nil
}
