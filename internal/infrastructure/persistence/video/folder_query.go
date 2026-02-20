package video

import (
	"context"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"

	"gorm.io/gorm"
)

// FolderQuery 收藏夹读模型查询的 GORM 实现。
type FolderQuery struct {
	db *gorm.DB
}

var _ app.FolderQuery = (*FolderQuery)(nil)

// NewFolderQuery 构造收藏夹查询。
func NewFolderQuery(db *gorm.DB) *FolderQuery {
	return &FolderQuery{db: db}
}

// ListWithCount 读取用户收藏夹列表（含视频数与「该视频是否已在此夹」）。
func (q *FolderQuery) ListWithCount(
	ctx context.Context,
	userID, videoID uint,
) ([]app.FolderWithCount, error) {
	var rows []app.FolderWithCount

	query := gormx.DB(ctx, q.db).
		Table("favorite_folders AS f").
		Joins("LEFT JOIN favorite_folder_videos ffv ON ffv.favorite_folder_id = f.id").
		Where("f.user_id = ?", userID)

	selectSQL := "f.id, f.name, f.is_default as is_default, " +
		"COUNT(ffv.video_id) as video_count, false as is_favorited"

	if videoID > 0 {
		query = query.Joins(
			"LEFT JOIN favorite_folder_videos fav ON fav.favorite_folder_id = f.id AND fav.video_id = ?",
			videoID,
		)
		selectSQL = "f.id, f.name, f.is_default as is_default, COUNT(ffv.video_id) as video_count, " +
			"CASE WHEN MAX(fav.video_id) IS NULL THEN false ELSE true END as is_favorited"
	}

	if err := query.
		Select(selectSQL).
		Group("f.id").
		Order("f.is_default DESC, f.id DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	return rows, nil
}

// ListVideos 读取收藏夹内的视频列表。
func (q *FolderQuery) ListVideos(
	ctx context.Context,
	folderID uint,
	page, pageSize, sort int,
) ([]app.FolderVideoItem, int64, error) {
	base := gormx.DB(ctx, q.db).
		Table("favorite_folder_videos AS ffv").
		Joins("JOIN videos v ON v.id = ffv.video_id").
		Joins("LEFT JOIN users u ON u.username = v.author_username").
		Where("ffv.favorite_folder_id = ? AND v.status = ?", folderID, enum.VideoStatusPublished)

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []app.FolderVideoItem{}, 0, nil
	}

	orderExpr := "IFNULL(ffv.created_at, v.created_at) DESC, ffv.video_id DESC"

	switch sort {
	case domain.FolderVideoSortViews:
		orderExpr = "v.views DESC, IFNULL(ffv.created_at, v.created_at) DESC, ffv.video_id DESC"
	case domain.FolderVideoSortLikes:
		orderExpr = "v.likes DESC, IFNULL(ffv.created_at, v.created_at) DESC, ffv.video_id DESC"
	}

	type row struct {
		ID            uint
		Title         string
		Cover         string
		Duration      int
		Views         int64
		DanmuCount    int64
		FavoriteCount int64
		FavoritedAt   time.Time
		AuthorID      uint
		AuthorName    string
		AuthorAvatar  string
	}

	var rows []row
	if err := base.Session(&gorm.Session{}).
		Select("v.id, v.title, v.cover, v.duration, v.views, v.danmu_count, v.favorite_count, " +
			"ffv.created_at as favorited_at, u.id as author_id, " +
			"v.author_username as author_name, u.avatar as author_avatar").
		Order(orderExpr).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]app.FolderVideoItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, app.FolderVideoItem{
			ID:            r.ID,
			Title:         r.Title,
			Cover:         r.Cover,
			Duration:      r.Duration,
			Views:         r.Views,
			DanmuCount:    r.DanmuCount,
			FavoriteCount: r.FavoriteCount,
			FavoritedAt:   r.FavoritedAt,
			Author: app.AuthorBrief{
				ID:       r.AuthorID,
				Username: r.AuthorName,
				Avatar:   r.AuthorAvatar,
			},
		})
	}

	return items, total, nil
}
