package video

import (
	"context"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// AdminVideoQuery 管理端视频列表读模型查询的 GORM 实现。
type AdminVideoQuery struct {
	db *gorm.DB
}

var _ app.AdminVideoQuery = (*AdminVideoQuery)(nil)

// NewAdminVideoQuery 构造管理端视频列表查询。
func NewAdminVideoQuery(db *gorm.DB) *AdminVideoQuery {
	return &AdminVideoQuery{db: db}
}

// List 管理端视频分页列表。
//
// AuthorUsername 由应用层按 UserID 解析后填充（读侧投影，见 decisions.md D-011）。
func (q *AdminVideoQuery) List(
	ctx context.Context,
	params app.AdminVideoListParams,
) ([]app.AdminVideoItem, int64, error) {
	query := gormx.DB(ctx, q.db).Model(&po.Video{})
	if params.IsScheduled != nil {
		query = query.Where("is_scheduled = ?", *params.IsScheduled)
	}

	if params.AuthorUsername != "" {
		query = query.Where("author_username = ?", params.AuthorUsername)
	}

	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		query = query.Where("title LIKE ? OR description LIKE ?", like, like)
	}

	if params.Status > 0 {
		query = query.Where("status = ?", params.Status)
	}

	if params.PartitionID > 0 {
		query = query.Where("partition_id = ?", params.PartitionID)
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []app.AdminVideoItem{}, 0, nil
	}

	var rows []po.Video
	if err := query.Session(&gorm.Session{}).
		Select("id", "title", "description", "cover", "duration", "views", "likes",
			"comment_count", "coin_count", "favorite_count", "danmu_count", "is_original",
			"storage_type", "status", "partition_id", "author_username", "created_at", "is_scheduled", "publish_at", "approved_at").
		Preload("Author", func(tx *gorm.DB) *gorm.DB {
			return tx.Select("id", "username", "avatar")
		}).
		Preload("VideoPartition", func(tx *gorm.DB) *gorm.DB {
			return tx.Select("id", "name")
		}).
		Order(adminVideoOrderSQL(params.Sort)).
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]app.AdminVideoItem, 0, len(rows))

	for i := range rows {
		v := rows[i]

		authorID := v.Author.ID

		authorName := v.Author.Username
		if authorName == "" {
			authorName = fallbackUsername(&v)
		}

		authorAvatar := v.Author.Avatar
		if authorID == 0 || authorName == "" {
			id, name := q.userByUsername(ctx, v.AuthorUsername)
			if authorID == 0 {
				authorID = id
			}

			if authorName == "" {
				authorName = name
			}
		}

		items = append(items, app.AdminVideoItem{
			IsScheduled:   v.IsScheduled,
			PublishAt:     v.PublishAt,
			ApprovedAt:    v.ApprovedAt,
			ID:            v.ID,
			Title:         v.Title,
			Cover:         v.Cover,
			Duration:      v.Duration,
			Views:         v.Views,
			Likes:         v.Likes,
			CommentCount:  v.CommentCount,
			CoinCount:     v.CoinCount,
			FavoriteCount: v.FavoriteCount,
			DanmuCount:    v.DanmuCount,
			IsOriginal:    v.IsOriginal,
			StorageType:   string(v.StorageType),
			Status:        v.Status,
			PartitionID:   v.PartitionID,
			PartitionName: v.VideoPartition.Name,
			Author: app.AdminVideoAuthor{
				ID:       authorID,
				Username: authorName,
				Avatar:   authorAvatar,
			},
			CreatedAt: v.CreatedAt,
		})
	}

	return items, total, nil
}

// userByUsername 兜底解析作者 ID / 用户名（与原 fetchUserByUsername 一致）。
func (q *AdminVideoQuery) userByUsername(ctx context.Context, username string) (uint, string) {
	if username == "" {
		return 0, ""
	}

	var user po.User
	if err := gormx.DB(ctx, q.db).
		Select("id", "username").
		Where("username = ?", username).
		First(&user).Error; err != nil {
		return 0, username
	}

	return user.ID, user.Username
}

// adminVideoOrderSQL 管理端列表排序表达式（与原 switch 分支逐字一致）。
func adminVideoOrderSQL(sort string) string {
	switch sort {
	case "oldest":
		return "created_at ASC, id ASC"
	case "views":
		return "views DESC, created_at DESC, id DESC"
	case "likes":
		return "likes DESC, created_at DESC, id DESC"
	case "duration":
		return "duration DESC, created_at DESC, id DESC"
	default:
		return "created_at DESC, id DESC"
	}
}
