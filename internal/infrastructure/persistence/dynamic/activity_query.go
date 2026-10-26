package dynamic

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"time"

	app "f_ccg_video_back_ddd/internal/application/dynamic"
	domain "f_ccg_video_back_ddd/internal/domain/dynamic"

	"gorm.io/gorm"
)

// ActivityQuery 作品活跃度与已读位点查询的 GORM 实现。
//
// videos 表属于 video 上下文、users 表属于 identity 上下文，
// 此处按 D-011 仅做读侧投影；dynamics / dynamic_reads 为本上下文自有表。
type ActivityQuery struct {
	db *gorm.DB
}

var _ app.ActivityQuery = (*ActivityQuery)(nil)

// NewActivityQuery 构造作品活跃度查询。
func NewActivityQuery(db *gorm.DB) *ActivityQuery {
	return &ActivityQuery{db: db}
}

// LatestVideoTimes 批量读取作者最新已发布视频时间（key 为作者用户名）。
func (q *ActivityQuery) LatestVideoTimes(
	ctx context.Context,
	authorUsernames []string,
) (map[string]time.Time, error) {
	if len(authorUsernames) == 0 {
		return map[string]time.Time{}, nil
	}

	type row struct {
		AuthorUsername string    `gorm:"column:author_username"`
		MaxAt          time.Time `gorm:"column:max_at"`
	}

	var rows []row
	if err := gormx.DB(ctx, q.db).
		Table("videos").
		Select("author_username, MAX(created_at) as max_at").
		Where("status = ? AND author_username IN ?", enum.VideoStatusPublished, authorUsernames).
		Group("author_username").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	m := make(map[string]time.Time, len(rows))
	for _, r := range rows {
		m[r.AuthorUsername] = r.MaxAt
	}

	return m, nil
}

// LatestDynamicTimes 批量读取作者最新动态时间（key 为作者用户 ID）。
func (q *ActivityQuery) LatestDynamicTimes(ctx context.Context, authorIDs []uint) (map[uint]time.Time, error) {
	if len(authorIDs) == 0 {
		return map[uint]time.Time{}, nil
	}

	type row struct {
		UserID uint      `gorm:"column:user_id"`
		MaxAt  time.Time `gorm:"column:max_at"`
	}

	var rows []row
	if err := gormx.DB(ctx, q.db).
		Table("dynamics").
		Select("user_id, MAX(created_at) as max_at").
		Where("user_id IN ?", authorIDs).
		Group("user_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	m := make(map[uint]time.Time, len(rows))
	for _, r := range rows {
		m[r.UserID] = r.MaxAt
	}

	return m, nil
}

// LastReadTimes 批量读取当前用户对各作者的已读位点时间。
func (q *ActivityQuery) LastReadTimes(
	ctx context.Context,
	userID uint,
	authorIDs []uint,
) (map[uint]time.Time, error) {
	if userID == 0 || len(authorIDs) == 0 {
		return map[uint]time.Time{}, nil
	}

	type row struct {
		AuthorID uint      `gorm:"column:author_id"`
		ReadAt   time.Time `gorm:"column:last_read_work_at"`
	}

	var rows []row
	if err := gormx.DB(ctx, q.db).
		Table("dynamic_reads").
		Select("author_id, last_read_work_at").
		Where("user_id = ? AND author_id IN ?", userID, authorIDs).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	m := make(map[uint]time.Time, len(rows))
	for _, r := range rows {
		m[r.AuthorID] = r.ReadAt
	}

	return m, nil
}

// simpleWorkRow 作品明细行（仅 ID 与发布时间）。
type simpleWorkRow struct {
	ID        uint      `gorm:"column:id"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

// LatestVideoWork 读取作者最新一条已发布视频；不存在时返回零值 WorkRef。
func (q *ActivityQuery) LatestVideoWork(ctx context.Context, authorUsername string) (domain.WorkRef, error) {
	var row simpleWorkRow
	if err := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Select("id, created_at").
		Where("author_username = ? AND status = ?", authorUsername, enum.VideoStatusPublished).
		Order("created_at DESC, id DESC").
		Limit(1).
		Scan(&row).Error; err != nil {
		return domain.WorkRef{}, err
	}

	return domain.WorkRef{ID: row.ID, CreatedAt: row.CreatedAt}, nil
}

// LatestDynamicWork 读取作者最新一条动态；不存在时返回零值 WorkRef。
func (q *ActivityQuery) LatestDynamicWork(ctx context.Context, authorID uint) (domain.WorkRef, error) {
	var row simpleWorkRow
	if err := gormx.DB(ctx, q.db).
		Model(&po.Dynamic{}).
		Select("id, created_at").
		Where("user_id = ?", authorID).
		Order("created_at DESC, id DESC").
		Limit(1).
		Scan(&row).Error; err != nil {
		return domain.WorkRef{}, err
	}

	return domain.WorkRef{ID: row.ID, CreatedAt: row.CreatedAt}, nil
}

// PublishedVideoAuthor 读取已发布视频及其作者引用；视频不存在或未发布时返回零值。
func (q *ActivityQuery) PublishedVideoAuthor(ctx context.Context, videoID uint) (app.VideoAuthorRef, error) {
	type row struct {
		VideoID     uint      `gorm:"column:video_id"`
		VideoAt     time.Time `gorm:"column:video_at"`
		AuthorID    uint      `gorm:"column:author_id"`
		AuthorName  string    `gorm:"column:author_username"`
		VideoStatus int       `gorm:"column:status"`
	}

	var r row

	err := gormx.DB(ctx, q.db).
		Table("videos v").
		Select("v.id as video_id, v.created_at as video_at, v.status as status, u.id as author_id, v.author_username as author_username").
		Joins("JOIN users u ON u.username = v.author_username").
		Where("v.id = ? AND v.status = ?", videoID, enum.VideoStatusPublished).
		Limit(1).
		Scan(&r).Error
	if err != nil {
		return app.VideoAuthorRef{}, err
	}

	return app.VideoAuthorRef{
		VideoID:        r.VideoID,
		VideoAt:        r.VideoAt,
		AuthorID:       r.AuthorID,
		AuthorUsername: r.AuthorName,
		VideoStatus:    r.VideoStatus,
	}, nil
}
