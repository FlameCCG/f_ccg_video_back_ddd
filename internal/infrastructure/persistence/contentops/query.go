package contentops

import (
	"context"
	"time"

	app "f_ccg_video_back_ddd/internal/application/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// VideoPartitionQuery 提供视频分区存在性的只读 GORM 投影。
//
// 分区聚合归 video 上下文所有，这里使用 COUNT 校验。
type VideoPartitionQuery struct {
	db *gorm.DB
}

var _ app.VideoPartitionQuery = (*VideoPartitionQuery)(nil)

// NewVideoPartitionQuery 构造分区存在性查询。
func NewVideoPartitionQuery(db *gorm.DB) *VideoPartitionQuery {
	return &VideoPartitionQuery{db: db}
}

// Exists 报告分区是否存在。
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

// AdminStatsQuery 提供管理端站点统计的只读 GORM 投影。
//
// 统计直读 users、videos、comments 与 user_login_records，仅执行计数，不写入其它上下文。
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
