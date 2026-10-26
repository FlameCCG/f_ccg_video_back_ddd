package video

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"

	"gorm.io/gorm"
)

// HistoryQuery 观看历史列表读模型查询的 GORM 实现。
type HistoryQuery struct {
	db *gorm.DB
}

var _ app.HistoryQuery = (*HistoryQuery)(nil)

// NewHistoryQuery 构造观看历史查询。
func NewHistoryQuery(db *gorm.DB) *HistoryQuery {
	return &HistoryQuery{db: db}
}

// List 分页读取用户观看历史（仅已发布视频，支持标题/作者关键字过滤）。
func (q *HistoryQuery) List(
	ctx context.Context,
	userID uint,
	page, pageSize int,
	keyword string,
) ([]app.HistoryItem, int64, error) {
	base := gormx.DB(ctx, q.db).
		Table("video_play_histories AS h").
		Joins("JOIN videos v ON v.id = h.video_id").
		Joins("LEFT JOIN users u ON u.username = v.author_username").
		Select("h.video_id, h.progress, h.duration, h.created_at, v.title, v.cover, "+
			"v.duration AS video_duration, v.author_username, u.id AS author_id").
		Where("h.user_id = ? AND v.status = ?", userID, enum.VideoStatusPublished)

	if keyword != "" {
		likeKeyword := "%" + keyword + "%"
		base = base.Where("v.title LIKE ? OR v.author_username LIKE ?", likeKeyword, likeKeyword)
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []app.HistoryItem{}, 0, nil
	}

	type row struct {
		VideoID        uint
		Progress       int
		Duration       int
		CreatedAt      time.Time
		Title          string
		Cover          string
		VideoDuration  int
		AuthorUsername string
		AuthorID       uint
	}

	var rows []row
	if err := base.Session(&gorm.Session{}).
		Order("h.created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]app.HistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, app.HistoryItem{
			VideoID:   r.VideoID,
			Title:     r.Title,
			Cover:     r.Cover,
			Duration:  r.VideoDuration,
			Progress:  r.Progress,
			CreatedAt: r.CreatedAt,
			AuthorID:  r.AuthorID,
			Author:    r.AuthorUsername,
		})
	}

	return items, total, nil
}
