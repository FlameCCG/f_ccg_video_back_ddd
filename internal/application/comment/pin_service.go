package comment

import (
	"context"
	"time"

	commenterrors "f_ccg_video_back_ddd/internal/domain/comment/domainerr"
	commentrepo "f_ccg_video_back_ddd/internal/domain/comment/repository"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
)

// Pin 置顶/取消置顶评论（仅作品作者）。
func (s *PinService) Pin(ctx context.Context, userID, commentID uint, pinned bool) error {
	if userID == 0 {
		return commenterrors.ErrUserNotLogin
	}

	if commentID == 0 {
		return commenterrors.ErrCommentIDRequired
	}

	target, err := s.comments.FindPinTarget(ctx, commentID)
	if err != nil {
		return err
	}

	if target == nil {
		return commenterrors.ErrCommentNotFound
	}

	if err := target.EnsurePinnable(); err != nil {
		return err
	}

	var isAuthor bool

	if target.VideoID > 0 {
		isAuthor, err = isVideoAuthor(ctx, s.related, userID, target.VideoID)
	} else {
		isAuthor, err = isDynamicAuthor(ctx, s.related, userID, target.DynamicID)
	}

	if err != nil {
		return err
	}

	if !isAuthor {
		return commenterrors.ErrPinForbidden
	}

	if err := target.SetPinned(pinned, time.Now()); err != nil {
		return err
	}

	return s.txm.Do(ctx, func(txCtx context.Context) error {
		if !target.IsPinned {
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

		return s.comments.SetPinned(txCtx, target.ID, target.IsPinned, target.PinnedAt)
	})
}

// PinService 负责置顶授权与事务内替换。
type PinService struct {
	txm      tx.Manager
	comments commentrepo.Repository
	related  RelatedQuery
}

func NewPinService(txm tx.Manager, comments commentrepo.Repository, related RelatedQuery) *PinService {
	return &PinService{txm: txm, comments: comments, related: related}
}
