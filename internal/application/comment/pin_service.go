package comment

import (
	"context"
	"time"

	commentdomain "f_ccg_video_back_ddd/internal/domain/comment"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
)

// Pin 置顶/取消置顶评论（仅作品作者）。
func (s *PinService) Pin(ctx context.Context, userID, commentID uint, pinned bool) error {
	if err := s.pins.Require(userID, commentID); err != nil {
		return err
	}

	return s.txm.Do(ctx, func(txCtx context.Context) error {
		// 锁住最新目标，避免并发删除后清空旧置顶却未能设置新置顶。
		target, err := s.comments.FindPinTargetForUpdate(txCtx, commentID)
		if err != nil {
			return err
		}
		if target == nil {
			return commentdomain.ErrCommentNotFound
		}
		if err := target.EnsurePinnable(); err != nil {
			return err
		}

		var isAuthor bool
		if target.VideoID() > 0 {
			isAuthor, err = (authorGate{related: s.related}).video(txCtx, userID, target.VideoID())
		} else {
			isAuthor, err = (authorGate{related: s.related}).dynamic(txCtx, userID, target.DynamicID())
		}
		if err != nil {
			return err
		}
		if err := target.PinBy(isAuthor, pinned, time.Now()); err != nil {
			return err
		}

		if !target.IsPinned() {
			return s.comments.SetPinned(txCtx, target.ID(), false, nil)
		}
		if target.VideoID() > 0 {
			if err := s.comments.UnpinByVideo(txCtx, target.VideoID()); err != nil {
				return err
			}
		} else if err := s.comments.UnpinByDynamic(txCtx, target.DynamicID()); err != nil {
			return err
		}
		return s.comments.SetPinned(txCtx, target.ID(), target.IsPinned(), target.PinnedAt())
	})
}

// PinService 负责置顶授权与事务内替换。
type PinService struct {
	txm      tx.Manager
	comments commentdomain.Repository
	related  RelatedQuery
	pins     commentdomain.PinPolicy
}

func NewPinService(txm tx.Manager, comments commentdomain.Repository, related RelatedQuery) *PinService {
	return &PinService{txm: txm, comments: comments, related: related, pins: commentdomain.PinPolicy{}}
}
