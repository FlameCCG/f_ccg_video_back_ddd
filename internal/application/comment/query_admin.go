package comment

import (
	"context"
	"strings"

	commentvo "f_ccg_video_back_ddd/internal/domain/comment/valueobject"
)

// AdminList 管理端评论列表。
//
// 管理端不需要 isLiked（视作未登录），但点赞数仍以 Redis 缓存为主。
func (s *QueryService) AdminList(ctx context.Context, params AdminListParams) ([]CommentItem, int64, error) {
	page, pageSize := commentvo.NormalizeAdminPaging(params.Page, params.PageSize)

	rows, total, err := s.query.AdminList(ctx, AdminListQuery{
		UserID:   params.UserID,
		Username: strings.TrimSpace(params.Username),
		Keyword:  strings.TrimSpace(params.Keyword),
		Sort:     params.Sort,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, 0, err
	}

	return s.buildCommentItems(ctx, rows, 0), total, nil
}
