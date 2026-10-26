// Package comment 实现评论写仓储、列表查询和关联清理。
//
// comment_repo.go 负责写侧，query.go/related_query.go 构建读侧，purge_repo.go 提供跨用例评论清理。
// converter.go 在 po 与领域/应用模型之间转换，数据库错误在适配器边界翻译为业务可识别错误。
//
// 仓储遵守消费方端口的不存在、错误和 ID 回填约定，不能只看方法名判断 nil 或错误的含义。
package comment

import (
	app "f_ccg_video_back_ddd/internal/application/comment"
	domain "f_ccg_video_back_ddd/internal/domain/comment"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
)

// toDomainComment 把评论 PO 转为领域实体（逐字段显式映射，不含关联对象）。
func toDomainComment(row *po.Comment) *domain.Comment {
	if row == nil {
		return nil
	}

	return &domain.Comment{
		ID:            row.ID,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
		Content:       row.Content,
		UserID:        row.UserID,
		VideoID:       row.VideoID,
		DynamicID:     row.DynamicID,
		CommentType:   row.CommentType,
		ParentID:      row.ParentID,
		RootID:        row.RootID,
		AtUserIDs:     row.AtUserIDs,
		Pictures:      row.Pictures,
		LikeCount:     row.LikeCount,
		ReplyCount:    row.ReplyCount,
		IsPinned:      row.IsPinned,
		PinnedAt:      row.PinnedAt,
		CommenterIP:   row.CommenterIP,
		CommenterAddr: row.CommenterAddr,
	}
}

// toDomainComments 批量转换评论 PO（用于只读投影列表）。
func toDomainComments(rows []po.Comment) []domain.Comment {
	list := make([]domain.Comment, 0, len(rows))

	for i := range rows {
		if c := toDomainComment(&rows[i]); c != nil {
			list = append(list, *c)
		}
	}

	return list
}

// toPOComment 把评论领域实体转为 PO（仅标量字段，关联对象由 GORM 自行处理）。
func toPOComment(c *domain.Comment) *po.Comment {
	if c == nil {
		return nil
	}

	return &po.Comment{
		Model: po.Model{
			ID:        c.ID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		},
		Content:       c.Content,
		UserID:        c.UserID,
		VideoID:       c.VideoID,
		DynamicID:     c.DynamicID,
		CommentType:   c.CommentType,
		ParentID:      c.ParentID,
		RootID:        c.RootID,
		AtUserIDs:     c.AtUserIDs,
		Pictures:      c.Pictures,
		LikeCount:     c.LikeCount,
		ReplyCount:    c.ReplyCount,
		IsPinned:      c.IsPinned,
		PinnedAt:      c.PinnedAt,
		CommenterIP:   c.CommenterIP,
		CommenterAddr: c.CommenterAddr,
	}
}

// toDomainLike 把评论点赞 PO 转为领域实体。
func toDomainLike(row *po.CommentLike) *domain.Like {
	if row == nil {
		return nil
	}

	return &domain.Like{
		ID:        row.ID,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		CommentID: row.CommentID,
		UserID:    row.UserID,
	}
}

// toPOLike 把评论点赞领域实体转为 PO。
func toPOLike(like *domain.Like) *po.CommentLike {
	if like == nil {
		return nil
	}

	return &po.CommentLike{
		Model: po.Model{
			ID:        like.ID,
			CreatedAt: like.CreatedAt,
			UpdatedAt: like.UpdatedAt,
		},
		CommentID: like.CommentID,
		UserID:    like.UserID,
	}
}

// toCommentRow 把带关联预加载的评论 PO 组装为应用层读模型。
//
// 字段取值与原 CommentService.buildCommentItems 逐项一致：
// 未预加载的关联（如列表接口不预加载 Video）取零值，由 json omitempty 省略；
// ReplyTo 仅在父评论及其用户都已预加载时填充。
func toCommentRow(row *po.Comment) app.Row {
	item := app.Row{
		ID:            row.ID,
		Content:       row.Content,
		UserID:        row.UserID,
		Username:      row.User.Username,
		Avatar:        row.User.Avatar,
		VideoID:       row.VideoID,
		DynamicID:     row.DynamicID,
		VideoTitle:    row.Video.Title,
		VideoCover:    row.Video.Cover,
		ParentID:      row.ParentID,
		RootID:        row.RootID,
		AtUserIDs:     row.AtUserIDs,
		Pictures:      row.Pictures,
		LikeCount:     row.LikeCount,
		ReplyCount:    row.ReplyCount,
		IsPinned:      row.IsPinned,
		PinnedAt:      row.PinnedAt,
		CommenterAddr: row.CommenterAddr,
		CreatedAt:     row.CreatedAt,
	}
	// 如果有父评论，添加回复信息
	if row.ParentComment != nil && row.ParentComment.User.ID > 0 {
		item.ReplyTo = &app.ReplyInfo{
			UserID:   row.ParentComment.UserID,
			Username: row.ParentComment.User.Username,
		}
	}

	return item
}

// toCommentRows 批量组装评论读模型。
func toCommentRows(rows []po.Comment) []app.Row {
	items := make([]app.Row, len(rows))
	for i := range rows {
		items[i] = toCommentRow(&rows[i])
	}

	return items
}
