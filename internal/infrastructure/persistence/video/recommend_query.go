package video

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	app "f_ccg_video_back_ddd/internal/application/video"
	domain "f_ccg_video_back_ddd/internal/domain/video"

	"gorm.io/gorm"
)

// 推荐候选排序表达式（SQL 与原 recommend_service.go 逐字一致）。
const (
	// relatedHotScoreSQL 带表别名，用于 JOIN videos v 的查询。
	relatedHotScoreSQL = "COALESCE(v.views,0) + COALESCE(v.likes,0)*2 + " +
		"COALESCE(v.coin_count,0)*3 + COALESCE(v.favorite_count,0)*2"
	// hotScoreSQL 不带表别名，用于直接查 videos 表的查询。
	hotScoreSQL = "COALESCE(views,0) + COALESCE(likes,0)*2 + " +
		"COALESCE(coin_count,0)*3 + COALESCE(favorite_count,0)*2"
)

// RecommendQuery 相关推荐候选集查询的 GORM 实现。
type RecommendQuery struct {
	db *gorm.DB
}

var _ app.RecommendQuery = (*RecommendQuery)(nil)

// NewRecommendQuery 构造推荐候选查询。
func NewRecommendQuery(db *gorm.DB) *RecommendQuery {
	return &RecommendQuery{db: db}
}

// BaseVideo 读取推荐基准视频的分区；视频不存在或未发布时返回 domain.ErrVideoNotPublished。
func (q *RecommendQuery) BaseVideo(ctx context.Context, videoID uint) (uint, error) {
	var row po.Video
	if err := gormx.DB(ctx, q.db).
		Select("id", "partition_id").
		Where("id = ? AND status = ?", videoID, enum.VideoStatusPublished).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, domain.ErrVideoNotPublished
		}

		return 0, err
	}

	return row.PartitionID, nil
}

// TagIDs 读取视频的标签 ID 列表。
//
// 与原实现一致：查询错误被忽略，返回可用结果。
func (q *RecommendQuery) TagIDs(ctx context.Context, videoID uint) []uint {
	var tagIDs []uint

	_ = gormx.DB(ctx, q.db).
		Table("video_tags").
		Select("tag_id").
		Where("video_id = ?", videoID).
		Pluck("tag_id", &tagIDs).Error

	return tagIDs
}

// TagRelatedIDs 按共同标签数排序取候选视频 ID。
func (q *RecommendQuery) TagRelatedIDs(
	ctx context.Context,
	videoID uint,
	tagIDs []uint,
	limit int,
) ([]uint, error) {
	if len(tagIDs) == 0 || limit <= 0 {
		return nil, nil
	}

	type row struct {
		VideoID uint `gorm:"column:video_id"`
	}

	var rows []row
	if err := gormx.DB(ctx, q.db).
		Table("video_tags as vt").
		Select("vt.video_id as video_id").
		Joins("JOIN videos v ON v.id = vt.video_id").
		Where("v.status = ?", enum.VideoStatusPublished).
		Where("vt.video_id <> ?", videoID).
		Where("vt.tag_id IN ?", tagIDs).
		Group("vt.video_id").
		Order("COUNT(DISTINCT vt.tag_id) DESC, " + relatedHotScoreSQL + " DESC, v.created_at DESC, v.id DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	ids := make([]uint, 0, len(rows))

	for _, item := range rows {
		if item.VideoID > 0 {
			ids = append(ids, item.VideoID)
		}
	}

	return ids, nil
}

// PartitionRelatedIDs 按同分区热度取候选视频 ID。
func (q *RecommendQuery) PartitionRelatedIDs(
	ctx context.Context,
	partitionID uint,
	limit int,
	exclude []uint,
) ([]uint, error) {
	if partitionID == 0 || limit <= 0 {
		return nil, nil
	}

	query := gormx.DB(ctx, q.db).Model(&po.Video{}).
		Select("id").
		Where("status = ? AND partition_id = ?", enum.VideoStatusPublished, partitionID)

	if len(exclude) > 0 {
		query = query.Where("id NOT IN ?", exclude)
	}

	var ids []uint
	if err := query.
		Order(hotScoreSQL+" DESC, created_at DESC, id DESC").
		Limit(limit).
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}

	return ids, nil
}

// HotRelatedIDs 按全局热度取候选视频 ID。
func (q *RecommendQuery) HotRelatedIDs(ctx context.Context, limit int, exclude []uint) ([]uint, error) {
	if limit <= 0 {
		return nil, nil
	}

	query := gormx.DB(ctx, q.db).Model(&po.Video{}).
		Select("id").
		Where("status = ?", enum.VideoStatusPublished)

	if len(exclude) > 0 {
		query = query.Where("id NOT IN ?", exclude)
	}

	var ids []uint
	if err := query.
		Order(hotScoreSQL+" DESC, created_at DESC, id DESC").
		Limit(limit).
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}

	return ids, nil
}

// FetchAnyStatusByIDs 按 ID 列表读取列表项（不过滤状态），返回 id → item 映射。
func (q *RecommendQuery) FetchAnyStatusByIDs(
	ctx context.Context,
	videoIDs []uint,
) (map[uint]app.FeedItem, error) {
	if len(videoIDs) == 0 {
		return map[uint]app.FeedItem{}, nil
	}

	var rows []po.Video
	if err := gormx.DB(ctx, q.db).
		Preload("Author").
		Where("id IN ?", videoIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	return toFeedItemMap(rows), nil
}
