package search

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	app "f_ccg_video_back_ddd/internal/application/search"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SuggestFallbackQuery 是 ES 不可用时的联想回退查询（原 service/search_service/fallback.go）。
//
// 三个维度都按「前缀命中优先，其次按最新 id」排序，并按候选值去重（GROUP BY）。
type SuggestFallbackQuery struct {
	db *gorm.DB
}

var _ app.SuggestFallbackQuery = (*SuggestFallbackQuery)(nil)

// NewSuggestFallbackQuery 构造联想回退查询（db 允许为 nil）。
func NewSuggestFallbackQuery(db *gorm.DB) *SuggestFallbackQuery {
	return &SuggestFallbackQuery{db: db}
}

// Available 报告数据库句柄是否就绪。
func (q *SuggestFallbackQuery) Available() bool {
	return q != nil && q.db != nil
}

// VideoTitles 取已发布视频标题。
func (q *SuggestFallbackQuery) VideoTitles(ctx context.Context, prefix string, size int) ([]string, error) {
	if !q.Available() || size <= 0 {
		return []string{}, nil
	}

	like := "%" + prefix + "%"
	prefixLike := prefix + "%"

	var titles []string
	if err := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Select("title").
		Where("status = ? AND title LIKE ?", enum.VideoStatusPublished, like).
		Group("title").
		// 前缀命中优先，其次按最新 id
		Order(clause.Expr{SQL: "CASE WHEN title LIKE ? THEN 0 ELSE 1 END", Vars: []any{prefixLike}}).
		Order("MAX(id) DESC").
		Limit(size).
		Pluck("title", &titles).Error; err != nil {
		return nil, err
	}

	return titles, nil
}

// Usernames 取用户名。
func (q *SuggestFallbackQuery) Usernames(ctx context.Context, prefix string, size int) ([]string, error) {
	if !q.Available() || size <= 0 {
		return []string{}, nil
	}

	like := "%" + prefix + "%"
	prefixLike := prefix + "%"

	var usernames []string
	if err := gormx.DB(ctx, q.db).
		Model(&po.User{}).
		Select("username").
		Where("username LIKE ?", like).
		Group("username").
		Order(clause.Expr{SQL: "CASE WHEN username LIKE ? THEN 0 ELSE 1 END", Vars: []any{prefixLike}}).
		Order("MAX(id) DESC").
		Limit(size).
		Pluck("username", &usernames).Error; err != nil {
		return nil, err
	}

	return usernames, nil
}

// TagNames 取挂在已发布视频上的标签名。
func (q *SuggestFallbackQuery) TagNames(ctx context.Context, prefix string, size int) ([]string, error) {
	if !q.Available() || size <= 0 {
		return []string{}, nil
	}

	like := "%" + prefix + "%"
	prefixLike := prefix + "%"

	var names []string
	if err := gormx.DB(ctx, q.db).
		Model(&po.Tag{}).
		Select("tags.name").
		Joins("JOIN video_tags ON video_tags.tag_id = tags.id").
		Joins("JOIN videos ON videos.id = video_tags.video_id AND videos.status = ?", enum.VideoStatusPublished).
		Where("tags.name LIKE ?", like).
		Group("tags.name").
		Order(clause.Expr{SQL: "CASE WHEN tags.name LIKE ? THEN 0 ELSE 1 END", Vars: []any{prefixLike}}).
		Order("MAX(videos.id) DESC").
		Limit(size).
		Pluck("name", &names).Error; err != nil {
		return nil, err
	}

	return names, nil
}
