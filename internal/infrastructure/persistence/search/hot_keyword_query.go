package search

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	app "f_ccg_video_back_ddd/internal/application/search"

	"gorm.io/gorm"
)

// HotKeywordQuery 是 Redis 热搜词榜为空时的回退查询（原 service/search_service/hot_keywords.go）。
//
// 采样按「播放量 → 点赞 → 创建时间 → id」排序，由应用层去重后内存分页。
type HotKeywordQuery struct {
	db *gorm.DB
}

var _ app.HotKeywordFallbackQuery = (*HotKeywordQuery)(nil)

// NewHotKeywordQuery 构造热词回退查询（db 允许为 nil）。
func NewHotKeywordQuery(db *gorm.DB) *HotKeywordQuery {
	return &HotKeywordQuery{db: db}
}

// Available 报告数据库句柄是否就绪。
func (q *HotKeywordQuery) Available() bool {
	return q != nil && q.db != nil
}

// HotVideoTitles 按播放量取热门已发布视频标题样本。
func (q *HotKeywordQuery) HotVideoTitles(ctx context.Context, sampleSize int) ([]app.HotVideoTitle, error) {
	if !q.Available() {
		return nil, nil
	}

	var rows []app.HotVideoTitle
	if err := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Select("title", "views").
		Where("status = ?", enum.VideoStatusPublished).
		Order("views DESC, likes DESC, created_at DESC, id DESC").
		Limit(sampleSize).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	return rows, nil
}
