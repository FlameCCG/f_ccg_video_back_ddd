package video

import (
	"context"
	"fmt"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// 列表排序表达式（SQL 与原 feed_service.go / hot_rank_service.go 逐字一致）。
const (
	// hotScoreOrderExpression 首页流热度排序（无时间衰减）。
	hotScoreOrderExpression = "COALESCE(videos.views,0) + COALESCE(videos.likes,0)*2 + " +
		"COALESCE(videos.coin_count,0)*3 + COALESCE(videos.favorite_count,0)*2"
	// hotScoreDecayExpression 综合热门排序（带时间衰减）。
	hotScoreDecayExpression = "(COALESCE(views,0) + COALESCE(likes,0)*3 + COALESCE(coin_count,0)*4 + " +
		"COALESCE(favorite_count,0)*4 + COALESCE(comment_count,0)*2 + COALESCE(danmu_count,0)) / " +
		"POW(TIMESTAMPDIFF(HOUR, created_at, NOW()) + 2, 1.5)"
	// rankScoreExpression 排行榜综合分排序。
	rankScoreExpression = "COALESCE(views,0) + COALESCE(likes,0)*3 + COALESCE(coin_count,0)*4 + " +
		"COALESCE(favorite_count,0)*4 + COALESCE(comment_count,0)*2 + COALESCE(danmu_count,0)"
)

// FeedQuery 首页/热门/排行/推荐的列表读模型查询的 GORM 实现。
type FeedQuery struct {
	db *gorm.DB
}

var _ app.FeedQuery = (*FeedQuery)(nil)

// NewFeedQuery 构造首页流查询。
func NewFeedQuery(db *gorm.DB) *FeedQuery {
	return &FeedQuery{db: db}
}

// PreferredPartitionIDs 依据观看历史统计用户的偏好分区（按次数降序）。
func (q *FeedQuery) PreferredPartitionIDs(ctx context.Context, userID uint) ([]uint, error) {
	if userID == 0 {
		return nil, nil
	}

	type row struct {
		PartitionID uint `gorm:"column:partition_id"`
		Count       int64
	}

	var rows []row

	if err := gormx.DB(ctx, q.db).
		Table("video_play_histories as h").
		Select("v.partition_id as partition_id, COUNT(*) as count").
		Joins("JOIN videos v ON v.id = h.video_id").
		Where("h.user_id = ? AND v.status = ?", userID, enum.VideoStatusPublished).
		Group("v.partition_id").
		Order("count DESC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("querying feed preferences: %w", err)
	}

	ids := make([]uint, 0, len(rows))

	for _, item := range rows {
		if item.PartitionID > 0 {
			ids = append(ids, item.PartitionID)
		}
	}

	return ids, nil
}

// FetchFeed 按分区过滤 + 历史去重拉取首页流；返回列表与总数。
func (q *FeedQuery) FetchFeed(
	ctx context.Context,
	partitionIDs, excludeVideoIDs []uint,
	page, pageSize int,
) ([]app.FeedItem, int64, error) {
	query := gormx.DB(ctx, q.db).Model(&po.Video{}).
		Where("videos.status = ?", enum.VideoStatusPublished)

	if len(partitionIDs) > 0 {
		query = query.Where("videos.partition_id IN ?", partitionIDs)
	}

	// 调用方已将历史限制在 maxHistoryWindow；受控 NOT IN 避免无界列表拖垮优化器。
	if len(excludeVideoIDs) > 0 {
		if len(excludeVideoIDs) > maxHistoryWindow {
			excludeVideoIDs = excludeVideoIDs[:maxHistoryWindow]
		}

		query = query.Where("videos.id NOT IN ?", excludeVideoIDs)
	}

	offset := max((page-1)*pageSize, 0)

	var rows []po.Video
	if err := query.Session(&gorm.Session{}).
		Joins("Author").
		Order(hotScoreOrderExpression + " DESC, videos.created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	// 第一页空集可直接确定 total=0，让个性化 fallback 省掉一次 COUNT 往返。
	if len(rows) == 0 && page == 1 {
		return []app.FeedItem{}, 0, nil
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	return toFeedItems(rows), total, nil
}

// FetchByIDs 按 ID 列表读取列表项（仅已发布），返回 id → item 映射。
func (q *FeedQuery) FetchByIDs(ctx context.Context, videoIDs []uint) (map[uint]app.FeedItem, error) {
	if len(videoIDs) == 0 {
		return map[uint]app.FeedItem{}, nil
	}

	var rows []po.Video
	if err := gormx.DB(ctx, q.db).
		Preload("Author").
		Where("id IN ? AND status = ?", videoIDs, enum.VideoStatusPublished).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	return toFeedItemMap(rows), nil
}

// FetchHotFromDB 用库内衰减热度表达式分页拉取综合热门。
func (q *FeedQuery) FetchHotFromDB(
	ctx context.Context,
	page, pageSize int,
) ([]app.FeedItem, int64, error) {
	query := gormx.DB(ctx, q.db).Model(&po.Video{}).
		Where("status = ?", enum.VideoStatusPublished)

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []app.FeedItem{}, 0, nil
	}

	offset := max((page-1)*pageSize, 0)

	var rows []po.Video
	if err := query.Session(&gorm.Session{}).
		Preload("Author").
		Order(hotScoreDecayExpression + " DESC, created_at DESC, id DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	return toFeedItems(rows), total, nil
}

// CountPublished 统计已发布视频总数。
func (q *FeedQuery) CountPublished(ctx context.Context) (int64, error) {
	var total int64
	if err := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Where("status = ?", enum.VideoStatusPublished).
		Count(&total).Error; err != nil {
		return 0, err
	}

	return total, nil
}

// FetchRankFromDB 用库内综合分表达式分页拉取排行榜。
func (q *FeedQuery) FetchRankFromDB(ctx context.Context, offset, limit int) ([]app.FeedItem, error) {
	var rows []po.Video
	if err := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Where("status = ?", enum.VideoStatusPublished).
		Select("id", "title", "cover", "duration", "views", "likes", "comment_count",
			"coin_count", "favorite_count", "danmu_count", "author_username", "created_at").
		Preload("Author").
		Order(rankScoreExpression + " DESC, created_at DESC, id DESC").
		Offset(offset).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	return toFeedItems(rows), nil
}

// toFeedItem 把视频 PO 转为列表项（逐字段对齐原 toFeedItem）。
func toFeedItem(row *po.Video) app.FeedItem {
	return app.FeedItem{
		ID:         row.ID,
		Title:      row.Title,
		Cover:      row.Cover,
		Duration:   row.Duration,
		Views:      row.Views,
		DanmuCount: row.DanmuCount,
		CreatedAt:  row.CreatedAt,
		Author: app.FeedAuthorBrief{
			ID:       row.Author.ID,
			Username: fallbackUsername(row),
			Avatar:   row.Author.Avatar,
		},
	}
}

// toFeedItems 批量转换列表项。
func toFeedItems(rows []po.Video) []app.FeedItem {
	items := make([]app.FeedItem, 0, len(rows))
	for i := range rows {
		items = append(items, toFeedItem(&rows[i]))
	}

	return items
}

// toFeedItemMap 批量转换为 id → item 映射。
func toFeedItemMap(rows []po.Video) map[uint]app.FeedItem {
	out := make(map[uint]app.FeedItem, len(rows))
	for i := range rows {
		out[rows[i].ID] = toFeedItem(&rows[i])
	}

	return out
}
