package video

import (
	"context"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"

	"gorm.io/gorm"
)

// DanmuQuery 弹幕读模型查询的 GORM 实现。
type DanmuQuery struct {
	db *gorm.DB
}

var _ app.DanmuQuery = (*DanmuQuery)(nil)

// NewDanmuQuery 构造弹幕查询。
func NewDanmuQuery(db *gorm.DB) *DanmuQuery {
	return &DanmuQuery{db: db}
}

// History 按播放时间区间读取历史弹幕。
func (q *DanmuQuery) History(
	ctx context.Context,
	videoID, partID uint,
	start, end, limit int,
) ([]app.DanmuDTO, error) {
	query := gormx.DB(ctx, q.db).
		Model(&po.Danmu{}).
		Where("video_id = ? AND video_part_id = ?", videoID, partID)

	if end > 0 {
		query = query.Where("time_offset BETWEEN ? AND ?", start, end)
	} else {
		query = query.Where("time_offset >= ?", start)
	}

	var rows []po.Danmu
	if err := query.
		Order("time_offset asc, id asc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]app.DanmuDTO, 0, len(rows))
	for i := range rows {
		result = append(result, toDanmuDTO(&rows[i]))
	}

	return result, nil
}

// RoomList 读取房间弹幕（分段模式按播放时间升序分页；历史模式按创建时间倒序取最近 N 条）。
func (q *DanmuQuery) RoomList(
	ctx context.Context,
	params app.DanmuRoomQueryParams,
) ([]app.DanmuRoomItem, int64, error) {
	query := gormx.DB(ctx, q.db).
		Model(&po.Danmu{}).
		Where("video_id = ? AND video_part_id = ?", params.VideoID, params.VideoPartID)

	if params.UseTimeRange {
		query = query.Where("time_offset >= ? AND time_offset < ?", params.Start, params.End)
	}

	if params.HasDate {
		if params.UseTimeRange {
			// 分段模式：限定在指定日期当天
			query = query.Where("created_at >= ? AND created_at < ?", params.DateStart, params.DateEnd)
		} else {
			// 历史弹幕：查询该日期当天及更早的弹幕
			query = query.Where("created_at < ?", params.DateEnd)
		}
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []app.DanmuRoomItem{}, 0, nil
	}

	var rows []po.Danmu

	if params.UseTimeRange {
		// 分段模式：按播放时间升序分页
		if err := query.Session(&gorm.Session{}).
			Order("time_offset asc, id asc").
			Offset((params.Page - 1) * params.PageSize).
			Limit(params.PageSize).
			Find(&rows).Error; err != nil {
			return nil, 0, err
		}
	} else {
		// 历史弹幕：按创建时间降序取最近 N 条，不分页
		if err := query.Session(&gorm.Session{}).
			Order("created_at desc, id desc").
			Limit(params.PageSize).
			Find(&rows).Error; err != nil {
			return nil, 0, err
		}
	}

	items := make([]app.DanmuRoomItem, 0, len(rows))
	for i := range rows {
		items = append(items, app.DanmuRoomItem{
			ID:          rows[i].ID,
			VideoID:     rows[i].VideoID,
			VideoPartID: rows[i].VideoPartID,
			UserID:      rows[i].UserID,
			Content:     rows[i].Content,
			TimeOffset:  rows[i].TimeOffset,
			Color:       rows[i].Color,
			FontSize:    rows[i].FontSize,
			Position:    rows[i].Position,
			LikeCount:   rows[i].LikeCount,
			IsLiked:     false,
			CreatedAt:   rows[i].CreatedAt,
		})
	}

	return items, total, nil
}

// MonthInstants 返回 [start, end) 内每条弹幕的 created_at，按时间升序。
func (q *DanmuQuery) MonthInstants(
	ctx context.Context,
	videoID, partID uint,
	start, end time.Time,
) ([]time.Time, error) {
	var instants []time.Time

	err := gormx.DB(ctx, q.db).
		Model(&po.Danmu{}).
		Where("video_id = ? AND video_part_id = ?", videoID, partID).
		Where("created_at >= ? AND created_at < ?", start, end).
		Order("created_at ASC, id ASC").
		Pluck("created_at", &instants).Error
	if err != nil {
		return nil, err
	}

	if instants == nil {
		return []time.Time{}, nil
	}

	return instants, nil
}

// AdminList 管理端弹幕列表。
func (q *DanmuQuery) AdminList(
	ctx context.Context,
	params app.AdminDanmuListParams,
) ([]app.AdminDanmuItem, int64, error) {
	query := gormx.DB(ctx, q.db).Model(&po.Danmu{})

	if params.UserID > 0 {
		query = query.Where("user_id = ?", params.UserID)
	}

	if params.Username != "" {
		query = query.Joins("JOIN users u ON u.id = danmu.user_id").
			Where("u.username LIKE ?", "%"+params.Username+"%")
	}

	if params.Keyword != "" {
		query = query.Where("content LIKE ?", "%"+params.Keyword+"%")
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []app.AdminDanmuItem{}, 0, nil
	}

	var rows []po.Danmu
	if err := query.Session(&gorm.Session{}).
		Preload("User", func(tx *gorm.DB) *gorm.DB {
			return tx.Select("id", "username", "avatar")
		}).
		Preload("Video", func(tx *gorm.DB) *gorm.DB {
			return tx.Select("id", "title", "cover")
		}).
		Order("created_at DESC, id DESC").
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]app.AdminDanmuItem, 0, len(rows))
	for _, d := range rows {
		items = append(items, app.AdminDanmuItem{
			ID:          d.ID,
			UserID:      d.UserID,
			Username:    d.User.Username,
			Avatar:      d.User.Avatar,
			VideoID:     d.VideoID,
			VideoTitle:  d.Video.Title,
			VideoCover:  d.Video.Cover,
			VideoPartID: d.VideoPartID,
			Content:     d.Content,
			TimeOffset:  d.TimeOffset,
			Color:       d.Color,
			FontSize:    d.FontSize,
			Position:    d.Position,
			LikeCount:   d.LikeCount,
			CreatedAt:   d.CreatedAt,
		})
	}

	return items, total, nil
}

// CreatorList 创作中心弹幕列表（按作者用户名过滤）。
func (q *DanmuQuery) CreatorList(
	ctx context.Context,
	authorUsername string,
	page, pageSize, sort int,
	keyword string,
) ([]app.CreatorDanmuItem, int64, error) {
	query := gormx.DB(ctx, q.db).
		Model(&po.Danmu{}).
		Joins("JOIN videos ON videos.id = danmu.video_id").
		Joins("JOIN users ON users.id = danmu.user_id").
		Preload("User", func(tx *gorm.DB) *gorm.DB {
			return tx.Select("id", "username", "avatar")
		}).
		Preload("Video", func(tx *gorm.DB) *gorm.DB {
			return tx.Select("id", "title", "cover")
		}).
		Where("videos.author_username = ?", authorUsername)

	if keyword != "" {
		likeKeyword := "%" + keyword + "%"
		query = query.Where(
			"danmu.content LIKE ? OR users.username LIKE ? OR videos.title LIKE ?",
			likeKeyword,
			likeKeyword,
			likeKeyword,
		)
	}

	switch sort {
	case 1:
		query = query.Order("danmu.like_count DESC, danmu.created_at DESC, danmu.id DESC")
	default:
		query = query.Order("danmu.created_at DESC, danmu.id DESC")
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []app.CreatorDanmuItem{}, 0, nil
	}

	var rows []po.Danmu
	if err := query.Session(&gorm.Session{}).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]app.CreatorDanmuItem, 0, len(rows))
	for _, d := range rows {
		items = append(items, app.CreatorDanmuItem{
			ID:          d.ID,
			UserID:      d.UserID,
			Username:    d.User.Username,
			Avatar:      d.User.Avatar,
			VideoID:     d.VideoID,
			VideoTitle:  d.Video.Title,
			VideoCover:  d.Video.Cover,
			VideoPartID: d.VideoPartID,
			Content:     d.Content,
			TimeOffset:  d.TimeOffset,
			Color:       d.Color,
			FontSize:    d.FontSize,
			Position:    d.Position,
			LikeCount:   d.LikeCount,
			CreatedAt:   d.CreatedAt,
		})
	}

	return items, total, nil
}

// toDanmuDTO 把弹幕 PO 转为响应 DTO。
func toDanmuDTO(row *po.Danmu) app.DanmuDTO {
	return app.DanmuDTO{
		ID:          row.ID,
		VideoID:     row.VideoID,
		VideoPartID: row.VideoPartID,
		UserID:      row.UserID,
		Content:     row.Content,
		TimeOffset:  row.TimeOffset,
		Color:       row.Color,
		FontSize:    row.FontSize,
		Position:    row.Position,
		LikeCount:   row.LikeCount,
		IsLiked:     false,
		CreatedAt:   row.CreatedAt,
	}
}
