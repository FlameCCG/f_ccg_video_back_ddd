package comment

import (
	"context"
	"errors"

	app "f_ccg_video_back_ddd/internal/application/comment"
	domain "f_ccg_video_back_ddd/internal/domain/comment"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// rootCommentWhere 一级评论的筛选条件（与原实现共用同一份 where 串）。
const rootCommentWhere = "root_id IS NULL AND video_id = ? AND dynamic_id = ?"

// Query 评论读侧投影的 GORM 实现。
type Query struct {
	db *gorm.DB
}

var _ app.Query = (*Query)(nil)

// NewQuery 构造评论读侧投影。
func NewQuery(db *gorm.DB) *Query {
	return &Query{db: db}
}

// selectUserProfile 预加载评论用户的精简列。
func selectUserProfile(db *gorm.DB) *gorm.DB {
	return db.Select("id", "username", "avatar")
}

// selectUserName 预加载父评论用户的精简列。
func selectUserName(db *gorm.DB) *gorm.DB {
	return db.Select("id", "username")
}

// ListRoots 一级评论分页列表（原 CommentService.List）。
func (q *Query) ListRoots(
	ctx context.Context,
	params app.RootListQuery,
) ([]app.Row, int64, error) {
	var total int64
	if err := gormx.DB(ctx, q.db).Model(&po.Comment{}).
		Where(rootCommentWhere, params.VideoID, params.DynamicID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := gormx.DB(ctx, q.db).
		Preload("User", selectUserProfile).
		Where(rootCommentWhere, params.VideoID, params.DynamicID)

	// 排序：置顶恒定优先
	switch params.SortBy {
	case "hot":
		query = query.Order("is_pinned DESC, pinned_at DESC, like_count DESC, created_at DESC")
	default:
		query = query.Order("is_pinned DESC, pinned_at DESC, created_at DESC")
	}

	var rows []po.Comment
	if err := query.
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	return toCommentRows(rows), total, nil
}

// ListReplies 某根评论下的回复分页列表（原 CommentService.Replies）。
func (q *Query) ListReplies(
	ctx context.Context,
	params app.ReplyListQuery,
) ([]app.Row, int64, error) {
	var total int64
	if err := gormx.DB(ctx, q.db).Model(&po.Comment{}).
		Where("root_id = ?", params.RootID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []po.Comment
	if err := gormx.DB(ctx, q.db).
		Preload("User", selectUserProfile).
		Preload("ParentComment.User", selectUserName).
		Where("root_id = ?", params.RootID).
		Order("created_at ASC").
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	return toCommentRows(rows), total, nil
}

// ListByAuthor 创作中心：本人视频下的评论分页列表（原 CommentService.ListByAuthor）。
func (q *Query) ListByAuthor(
	ctx context.Context,
	params app.CreatorListQuery,
) ([]app.Row, int64, error) {
	db := gormx.DB(ctx, q.db).
		Model(&po.Comment{}).
		Joins("JOIN videos ON videos.id = comments.video_id").
		Joins("JOIN users ON users.id = comments.user_id").
		Preload("User", selectUserProfile).
		Preload("ParentComment.User", selectUserName).
		Preload("Video", func(tx *gorm.DB) *gorm.DB {
			return tx.Select("id", "title", "cover", "author_username")
		}).
		Where("videos.author_username = ?", params.AuthorUsername)

	if params.Keyword != "" {
		likeKeyword := "%" + params.Keyword + "%"
		db = db.Where(
			"comments.content LIKE ? OR users.username LIKE ? OR videos.title LIKE ?",
			likeKeyword,
			likeKeyword,
			likeKeyword,
		)
	}

	switch params.Sort {
	case 1:
		db = db.Order("like_count DESC, created_at DESC")
	case 2:
		db = db.Order("reply_count DESC, created_at DESC")
	default:
		db = db.Order("created_at DESC")
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return nil, 0, nil
	}

	var rows []po.Comment
	if err := db.
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	return toCommentRows(rows), total, nil
}

// AdminList 管理端评论分页列表（原 admin_service.go AdminList）。
func (q *Query) AdminList(
	ctx context.Context,
	params app.AdminListQuery,
) ([]app.Row, int64, error) {
	query := gormx.DB(ctx, q.db).Model(&po.Comment{})
	if params.UserID > 0 {
		query = query.Where("user_id = ?", params.UserID)
	}

	if params.Username != "" {
		query = query.Joins("JOIN users u ON u.id = comments.user_id").
			Where("u.username LIKE ?", "%"+params.Username+"%")
	}

	if params.Keyword != "" {
		query = query.Where("content LIKE ?", "%"+params.Keyword+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return nil, 0, nil
	}

	var orderSQL string

	switch params.Sort {
	case "likes":
		orderSQL = "like_count DESC, created_at DESC, id DESC"
	case "replies":
		orderSQL = "reply_count DESC, created_at DESC, id DESC"
	case "oldest":
		orderSQL = "created_at ASC, id ASC"
	default: // latest
		orderSQL = "created_at DESC, id DESC"
	}

	var rows []po.Comment
	if err := query.
		Preload("User", selectUserProfile).
		Preload("Video", func(tx *gorm.DB) *gorm.DB {
			return tx.Select("id", "title", "cover")
		}).
		Preload("ParentComment", func(tx *gorm.DB) *gorm.DB {
			return tx.Select("id", "user_id")
		}).
		Preload("ParentComment.User", selectUserName).
		Order(orderSQL).
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	return toCommentRows(rows), total, nil
}

// CountByVideo 统计某视频的评论总数（含回复，原 CommentService.GetCount）。
func (q *Query) CountByVideo(ctx context.Context, videoID uint) (int64, error) {
	var count int64

	err := gormx.DB(ctx, q.db).Model(&po.Comment{}).
		Where("video_id = ?", videoID).Count(&count).Error

	return count, err
}

// ParentBrief 读取同一对象下的父评论投影；不存在时返回 (nil, nil)。
func (q *Query) ParentBrief(
	ctx context.Context,
	parentID, videoID, dynamicID uint,
) (*domain.ParentBrief, error) {
	var row struct {
		UserID   uint   `gorm:"column:user_id"`
		RootID   *uint  `gorm:"column:root_id"`
		Username string `gorm:"column:username"`
	}

	if err := gormx.DB(ctx, q.db).
		Table("comments c").
		Select("c.user_id, c.root_id, u.username").
		Joins("LEFT JOIN users u ON u.id = c.user_id").
		Where("c.id = ? AND c.video_id = ? AND c.dynamic_id = ?", parentID, videoID, dynamicID).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &domain.ParentBrief{
		UserID:   row.UserID,
		RootID:   row.RootID,
		Username: row.Username,
	}, nil
}
