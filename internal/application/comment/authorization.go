package comment

import (
	"context"

	commenterrors "f_ccg_video_back_ddd/internal/domain/comment/domainerr"
)

// isVideoAuthor 判断用户是否为视频作者（用于删除/置顶评论的授权）。
func isVideoAuthor(ctx context.Context, related RelatedQuery, userID, videoID uint) (bool, error) {
	ref, err := related.VideoAuthorRef(ctx, videoID)
	if err != nil {
		return false, err
	}

	if ref == nil {
		return false, commenterrors.ErrRecordNotFound
	}

	return ref.AuthorUserID == userID, nil
}

// isDynamicAuthor 判断用户是否为动态作者。
func isDynamicAuthor(ctx context.Context, related RelatedQuery, userID, dynamicID uint) (bool, error) {
	brief, err := related.DynamicBrief(ctx, dynamicID)
	if err != nil {
		return false, err
	}

	if brief == nil {
		return false, commenterrors.ErrRecordNotFound
	}

	return brief.UserID == userID, nil
}
