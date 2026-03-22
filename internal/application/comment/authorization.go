package comment

import (
	"context"

	commentdomain "f_ccg_video_back_ddd/internal/domain/comment"
)

// authorGate 用已读到的作品归属判断操作者是不是作者。
type authorGate struct {
	related RelatedQuery
}

func (g authorGate) video(ctx context.Context, userID, videoID uint) (bool, error) {
	ref, err := g.related.VideoAuthorRef(ctx, videoID)
	if err != nil {
		return false, err
	}

	if ref == nil {
		return false, commentdomain.ErrRecordNotFound
	}

	return (commentdomain.WorkAuthorship{AuthorID: ref.AuthorUserID}).OwnedBy(userID), nil
}

func (g authorGate) dynamic(ctx context.Context, userID, dynamicID uint) (bool, error) {
	brief, err := g.related.DynamicBrief(ctx, dynamicID)
	if err != nil {
		return false, err
	}

	if brief == nil {
		return false, commentdomain.ErrRecordNotFound
	}

	return (commentdomain.WorkAuthorship{AuthorID: brief.UserID}).OwnedBy(userID), nil
}
