package search

import (
	"context"

	app "f_ccg_video_back_ddd/internal/application/search"
	domain "f_ccg_video_back_ddd/internal/domain/search"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SearchFallbackQuery 是 ES 不可用时的搜索回退查询（原 service/search_service/fallback.go）。
//
// SQL 语句、排序表达式与 LIKE 模式与原实现逐字一致。
type SearchFallbackQuery struct {
	db *gorm.DB
}

var _ app.SearchFallbackQuery = (*SearchFallbackQuery)(nil)

// NewSearchFallbackQuery 构造搜索回退查询（db 允许为 nil）。
func NewSearchFallbackQuery(db *gorm.DB) *SearchFallbackQuery {
	return &SearchFallbackQuery{db: db}
}

// Available 报告数据库句柄是否就绪。
func (q *SearchFallbackQuery) Available() bool {
	return q != nil && q.db != nil
}

// SearchVideos 按标题/作者名模糊匹配已发布视频，返回一页命中与总量。
func (q *SearchFallbackQuery) SearchVideos(
	ctx context.Context,
	opts app.SearchOptions,
) ([]app.SearchVideoHit, int64, error) {
	if !q.Available() {
		return nil, 0, nil
	}

	keyword := domain.NormalizeKeyword(opts.Keyword)
	if keyword == "" {
		return nil, 0, nil
	}

	like := "%" + keyword + "%"
	prefix := keyword + "%"

	var total int64
	if err := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Where("status = ?", enum.VideoStatusPublished).
		Where("title LIKE ? OR author_username LIKE ?", like, like).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return nil, 0, nil
	}

	query := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Where("status = ?", enum.VideoStatusPublished).
		Where("title LIKE ? OR author_username LIKE ?", like, like).
		Preload("Tags")

	query = applyVideoFallbackSort(query, opts, keyword, prefix)

	offset := max((opts.Page-1)*opts.PageSize, 0)

	var videos []po.Video
	if err := query.Offset(offset).Limit(opts.PageSize).Find(&videos).Error; err != nil {
		return nil, 0, err
	}

	items := make([]app.SearchVideoHit, 0, len(videos))
	for _, video := range videos {
		items = append(items, toSearchVideoHit(video))
	}

	return items, total, nil
}

// SearchUsers 按用户名模糊匹配用户，返回一页命中与总量（粉丝数由子查询聚合）。
func (q *SearchFallbackQuery) SearchUsers(
	ctx context.Context,
	opts app.SearchOptions,
) ([]app.SearchUserHit, int64, error) {
	if !q.Available() {
		return nil, 0, nil
	}

	keyword := domain.NormalizeKeyword(opts.Keyword)
	if keyword == "" {
		return nil, 0, nil
	}

	like := "%" + keyword + "%"

	var total int64
	if err := gormx.DB(ctx, q.db).
		Model(&po.User{}).
		Where("username LIKE ?", like).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return nil, 0, nil
	}

	offset := max((opts.Page-1)*opts.PageSize, 0)

	query := gormx.DB(ctx, q.db).
		Table("users").
		Select("users.id, users.username, users.avatar, users.level, COALESCE(fc.cnt, 0) AS follower_count").
		Joins("LEFT JOIN (SELECT focus_user_id, COUNT(*) AS cnt FROM user_focus GROUP BY focus_user_id) fc ON fc.focus_user_id = users.id").
		Where("users.username LIKE ?", like)

	switch opts.UserSort {
	case domain.UserSortLevel:
		query = query.Order("users.level " + sqlSortDirection(opts.UserOrder))
	case domain.UserSortCreatedAt:
		query = query.Order("users.created_at " + sqlSortDirection(opts.UserOrder))
	default:
		query = query.Order("follower_count " + sqlSortDirection(opts.UserOrder))
	}

	query = query.Order("users.id DESC")

	var rows []userFallbackRow
	if err := query.Offset(offset).Limit(opts.PageSize).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	users := make([]app.SearchUserHit, 0, len(rows))
	for _, row := range rows {
		users = append(users, toSearchUserHit(row))
	}

	return users, total, nil
}

// CountUsers 统计用户名模糊命中总量。
func (q *SearchFallbackQuery) CountUsers(ctx context.Context, keyword string) (int64, error) {
	if !q.Available() {
		return 0, nil
	}

	keyword = domain.NormalizeKeyword(keyword)
	if keyword == "" {
		return 0, nil
	}

	var total int64

	err := gormx.DB(ctx, q.db).
		Model(&po.User{}).
		Where("username LIKE ?", "%"+keyword+"%").
		Count(&total).Error

	return total, err
}

// VideoMetaByIDs 批量读取视频作者名与创建时间，用于回填 ES 文档缺失字段。
func (q *SearchFallbackQuery) VideoMetaByIDs(ctx context.Context, ids []uint) ([]app.VideoSearchMeta, error) {
	if !q.Available() || len(ids) == 0 {
		return nil, nil
	}

	var rows []app.VideoSearchMeta
	if err := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Select("id", "author_username", "created_at").
		Where("id IN ?", ids).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	return rows, nil
}

// applyVideoFallbackSort 施加视频回退排序：非相关性排序按业务字段，相关性排序按字面命中优先级。
func applyVideoFallbackSort(query *gorm.DB, opts app.SearchOptions, keyword, prefix string) *gorm.DB {
	if query == nil {
		return nil
	}

	switch opts.VideoSort {
	case domain.VideoSortViews:
		query = query.Order("views " + sqlSortDirection(opts.VideoOrder))
	case domain.VideoSortDanmu:
		query = query.Order("danmu_count " + sqlSortDirection(opts.VideoOrder))
	case domain.VideoSortDuration:
		query = query.Order("duration " + sqlSortDirection(opts.VideoOrder))
	default:
		query = query.Order(clause.Expr{
			SQL: `CASE
WHEN title = ? THEN 0
WHEN title LIKE ? THEN 1
WHEN author_username = ? THEN 2
WHEN author_username LIKE ? THEN 3
ELSE 4
			END ASC`,
			Vars: []any{keyword, prefix, keyword, prefix},
		})
	}

	return query.Order("created_at DESC").Order("id DESC")
}

// sqlSortDirection 把排序方向枚举翻译成 SQL 关键字。
func sqlSortDirection(order int) string {
	if order == domain.SortOrderAsc {
		return "ASC"
	}

	return "DESC"
}
