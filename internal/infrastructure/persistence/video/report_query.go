package video

import (
	"context"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ReportQuery 管理端举报列表读模型查询的 GORM 实现。
type ReportQuery struct {
	db *gorm.DB
}

var _ app.ReportQuery = (*ReportQuery)(nil)

// NewReportQuery 构造举报列表查询。
func NewReportQuery(db *gorm.DB) *ReportQuery {
	return &ReportQuery{db: db}
}

// AdminVideoReports 管理端视频举报分页列表。
func (q *ReportQuery) AdminVideoReports(
	ctx context.Context,
	page, pageSize, status int,
) ([]app.AdminVideoReportItem, int64, error) {
	query := gormx.DB(ctx, q.db).Table("video_reports as r")
	if status > 0 {
		query = query.Where("r.status = ?", status)
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []app.AdminVideoReportItem{}, 0, nil
	}

	type row struct {
		ID               uint           `gorm:"column:id"`
		ReporterUsername string         `gorm:"column:reporter_username"`
		VideoID          uint           `gorm:"column:video_id"`
		Reason           string         `gorm:"column:reason"`
		Detail           string         `gorm:"column:detail"`
		ImageUrls        datatypes.JSON `gorm:"column:image_urls"`
		Status           int            `gorm:"column:status"`
		HandleNote       string         `gorm:"column:handle_note"`
		HandledBy        uint           `gorm:"column:handled_by"`
		HandledAt        *time.Time     `gorm:"column:handled_at"`
		CreatedAt        time.Time      `gorm:"column:created_at"`
		VideoTitle       string         `gorm:"column:video_title"`
		VideoCover       string         `gorm:"column:video_cover"`
	}

	var rows []row
	if err := query.Session(&gorm.Session{}).
		Select("r.id, r.reporter_username, r.video_id, r.reason, r.detail, r.image_urls, " +
			"r.status, r.handle_note, r.handled_by, r.handled_at, r.created_at, " +
			"v.title as video_title, v.cover as video_cover").
		Joins("LEFT JOIN videos v ON v.id = r.video_id").
		Order("r.created_at DESC, r.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]app.AdminVideoReportItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, app.AdminVideoReportItem{
			ID:               r.ID,
			ReporterUsername: r.ReporterUsername,
			VideoID:          r.VideoID,
			VideoTitle:       r.VideoTitle,
			VideoCover:       r.VideoCover,
			Reason:           r.Reason,
			Detail:           r.Detail,
			ImageUrls:        parseImageJSON(r.ImageUrls),
			Status:           r.Status,
			HandleNote:       r.HandleNote,
			HandledBy:        r.HandledBy,
			HandledAt:        r.HandledAt,
			CreatedAt:        r.CreatedAt,
		})
	}

	return items, total, nil
}

// AdminDanmuReports 管理端弹幕举报分页列表。
func (q *ReportQuery) AdminDanmuReports(
	ctx context.Context,
	page, pageSize, status int,
) ([]app.AdminDanmuReportItem, int64, error) {
	query := gormx.DB(ctx, q.db).Table("danmu_reports as r")
	if status > 0 {
		query = query.Where("r.status = ?", status)
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []app.AdminDanmuReportItem{}, 0, nil
	}

	type row struct {
		ID               uint       `gorm:"column:id"`
		ReporterUsername string     `gorm:"column:reporter_username"`
		VideoID          uint       `gorm:"column:video_id"`
		DanmuID          uint       `gorm:"column:danmu_id"`
		Reason           string     `gorm:"column:reason"`
		Detail           string     `gorm:"column:detail"`
		Status           int        `gorm:"column:status"`
		HandleNote       string     `gorm:"column:handle_note"`
		HandledBy        uint       `gorm:"column:handled_by"`
		HandledAt        *time.Time `gorm:"column:handled_at"`
		CreatedAt        time.Time  `gorm:"column:created_at"`
		VideoTitle       string     `gorm:"column:video_title"`
		VideoCover       string     `gorm:"column:video_cover"`
		DanmuContent     string     `gorm:"column:danmu_content"`
	}

	var rows []row
	if err := query.Session(&gorm.Session{}).
		Select("r.id, r.reporter_username, r.video_id, r.danmu_id, r.reason, r.detail, " +
			"r.status, r.handle_note, r.handled_by, r.handled_at, r.created_at, " +
			"v.title as video_title, v.cover as video_cover, d.content as danmu_content").
		Joins("LEFT JOIN danmu d ON d.id = r.danmu_id").
		Joins("LEFT JOIN videos v ON v.id = r.video_id").
		Order("r.created_at DESC, r.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]app.AdminDanmuReportItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, app.AdminDanmuReportItem{
			ID:               r.ID,
			ReporterUsername: r.ReporterUsername,
			VideoID:          r.VideoID,
			VideoTitle:       r.VideoTitle,
			VideoCover:       r.VideoCover,
			DanmuID:          r.DanmuID,
			DanmuContent:     r.DanmuContent,
			Reason:           r.Reason,
			Detail:           r.Detail,
			Status:           r.Status,
			HandleNote:       r.HandleNote,
			HandledBy:        r.HandledBy,
			HandledAt:        r.HandledAt,
			CreatedAt:        r.CreatedAt,
		})
	}

	return items, total, nil
}
