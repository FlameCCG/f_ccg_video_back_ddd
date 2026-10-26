package contentops

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"time"

	app "f_ccg_video_back_ddd/internal/application/contentops"

	"gorm.io/gorm"
)

// VideoPartitionQuery 视频分区存在性查询的 GORM 实现。
//
// 跨上下文读侧投影（decisions.md D-011）：分区聚合归 video 上下文，
// 这里只做一次 COUNT 校验，避免为一次计数引入跨上下文调用链。
type VideoPartitionQuery struct {
	db *gorm.DB
}

var _ app.VideoPartitionQuery = (*VideoPartitionQuery)(nil)

// NewVideoPartitionQuery 构造分区存在性查询。
func NewVideoPartitionQuery(db *gorm.DB) *VideoPartitionQuery {
	return &VideoPartitionQuery{db: db}
}

// Exists 报告分区是否存在（原 api/banner_api/helper.go 的 COUNT 校验）。
func (q *VideoPartitionQuery) Exists(ctx context.Context, partitionID uint) (bool, error) {
	var count int64
	if err := gormx.DB(ctx, q.db).
		Model(&po.VideoPartition{}).
		Where("id = ?", partitionID).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

// AdminStatsQuery 管理端站点统计读侧投影的 GORM 实现。
//
// 涉及的 users / videos / comments / user_login_records 分属 identity、video、comment 上下文，
// 按 decisions.md D-011 允许在消费方上下文直读（纯 COUNT，无写侧语义）。
// 每个方法的 SQL 与原 service/site_service/admin_stats_service.go 逐条对应。
type AdminStatsQuery struct {
	db *gorm.DB
}

var _ app.AdminStatsQuery = (*AdminStatsQuery)(nil)

// NewAdminStatsQuery 构造管理端站点统计查询。
func NewAdminStatsQuery(db *gorm.DB) *AdminStatsQuery {
	return &AdminStatsQuery{db: db}
}

// CountUsers 统计用户总数。
func (q *AdminStatsQuery) CountUsers(ctx context.Context) (int64, error) {
	var total int64

	err := gormx.DB(ctx, q.db).Model(&po.User{}).Count(&total).Error

	return total, err
}

// CountPublishedVideos 统计已发布视频总数。
func (q *AdminStatsQuery) CountPublishedVideos(ctx context.Context) (int64, error) {
	var total int64

	err := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Where("status = ?", enum.VideoStatusPublished).
		Count(&total).Error

	return total, err
}

// CountComments 统计评论总数。
func (q *AdminStatsQuery) CountComments(ctx context.Context) (int64, error) {
	var total int64

	err := gormx.DB(ctx, q.db).Model(&po.Comment{}).Count(&total).Error

	return total, err
}

// CountUsersCreatedBetween 统计 [start, end) 区间内注册的用户数。
func (q *AdminStatsQuery) CountUsersCreatedBetween(
	ctx context.Context,
	start, end time.Time,
) (int64, error) {
	var total int64

	err := gormx.DB(ctx, q.db).
		Model(&po.User{}).
		Where("created_at >= ? AND created_at < ?", start, end).
		Count(&total).Error

	return total, err
}

// CountDistinctLoginUsersBetween 统计 [start, end) 区间内去重的登录用户数。
func (q *AdminStatsQuery) CountDistinctLoginUsersBetween(
	ctx context.Context,
	start, end time.Time,
) (int64, error) {
	var total int64

	err := gormx.DB(ctx, q.db).
		Model(&po.UserLoginRecord{}).
		Distinct("user_id").
		Where("created_at >= ? AND created_at < ?", start, end).
		Count(&total).Error

	return total, err
}

// CountPublishedVideosCreatedBetween 统计 [start, end) 区间内创建的已发布视频数。
func (q *AdminStatsQuery) CountPublishedVideosCreatedBetween(
	ctx context.Context,
	start, end time.Time,
) (int64, error) {
	var total int64

	err := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Where("status = ? AND created_at >= ? AND created_at < ?", enum.VideoStatusPublished, start, end).
		Count(&total).Error

	return total, err
}
