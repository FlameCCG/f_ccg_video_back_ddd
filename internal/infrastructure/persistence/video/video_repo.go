package video

import (
	"context"
	"errors"
	"fmt"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// VideoRepository 视频聚合写侧仓储的 GORM 实现。
type VideoRepository struct {
	db *gorm.DB
}

// 编译期断言：满足领域侧仓储接口。
var _ domain.VideoRepository = (*VideoRepository)(nil)

// NewVideoRepository 构造视频仓储。
func NewVideoRepository(db *gorm.DB) *VideoRepository {
	return &VideoRepository{db: db}
}

// FindByID 按主键读取视频；不存在时返回 (nil, nil)。
func (r *VideoRepository) FindByID(ctx context.Context, videoID uint) (*domain.Video, error) {
	var row po.Video
	if err := gormx.DB(ctx, r.db).
		Select("id", "author_username", "status", "partition_id", "title", "cover", "duration",
			"description", "views", "likes", "comment_count", "coin_count", "favorite_count",
			"danmu_count", "is_original", "storage_type", "created_at", "updated_at").
		Where("id = ?", videoID).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomainVideo(&row), nil
}

// FindByIDAndAuthor 按主键 + 作者用户名读取视频；不存在时返回 ErrRecordNotFound。
func (r *VideoRepository) FindByIDAndAuthor(
	ctx context.Context,
	videoID uint,
	authorUsername string,
) (*domain.Video, error) {
	var row po.Video
	if err := gormx.DB(ctx, r.db).
		Take(&row, "id = ? AND author_username = ?", videoID, authorUsername).Error; err != nil {
		return nil, mapNotFound(err)
	}

	return toDomainVideo(&row), nil
}

// EnsurePublished 校验视频存在且处于已发布状态；否则返回 ErrRecordNotFound。
func (r *VideoRepository) EnsurePublished(ctx context.Context, videoID uint) error {
	var row struct {
		ID uint
	}

	err := gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Select("id").
		Where("id = ? AND status = ?", videoID, enum.VideoStatusPublished).
		Limit(1).
		Take(&row).Error

	return mapNotFound(err)
}

// Exists 判断视频行是否存在（不限状态）。
func (r *VideoRepository) Exists(ctx context.Context, videoID uint) (bool, error) {
	var count int64
	if err := gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Where("id = ?", videoID).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

// PublishedStats 读取已发布视频的统计行；不存在时返回 ErrRecordNotFound。
func (r *VideoRepository) PublishedStats(ctx context.Context, videoID uint) (*domain.Stats, error) {
	var row po.Video
	if err := gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Select("views", "likes", "comment_count", "coin_count", "favorite_count", "danmu_count").
		Where("id = ? AND status = ?", videoID, enum.VideoStatusPublished).
		Take(&row).Error; err != nil {
		return nil, mapNotFound(err)
	}

	return toDomainStats(&row), nil
}

// ApplyStatsDelta 在 MySQL 上以「不小于 0」表达式应用统计增量（仅已发布视频）。
func (r *VideoRepository) ApplyStatsDelta(ctx context.Context, videoID uint, delta domain.StatsDelta) error {
	updates := statsDeltaUpdates(delta)
	if len(updates) == 0 {
		return nil
	}

	return gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Where("id = ? AND status = ?", videoID, enum.VideoStatusPublished).
		Updates(updates).Error
}

// statsDeltaUpdates 把统计增量转为「不小于 0」的列更新表达式（与原 dbUpdates 逐列一致）。
func statsDeltaUpdates(delta domain.StatsDelta) map[string]any {
	updates := make(map[string]any, 6)
	if delta.Views != 0 {
		updates["views"] = nonNegativeDeltaExpr("views", delta.Views)
	}

	if delta.Likes != 0 {
		updates["likes"] = nonNegativeDeltaExpr("likes", delta.Likes)
	}

	if delta.CommentCount != 0 {
		updates["comment_count"] = nonNegativeDeltaExpr("comment_count", delta.CommentCount)
	}

	if delta.CoinCount != 0 {
		updates["coin_count"] = nonNegativeDeltaExpr("coin_count", delta.CoinCount)
	}

	if delta.FavoriteCount != 0 {
		updates["favorite_count"] = nonNegativeDeltaExpr("favorite_count", delta.FavoriteCount)
	}

	if delta.DanmuCount != 0 {
		updates["danmu_count"] = nonNegativeDeltaExpr("danmu_count", delta.DanmuCount)
	}

	return updates
}

// nonNegativeDeltaExpr 生成「结果不小于 0」的增量表达式（SQL 与原实现逐字一致）。
func nonNegativeDeltaExpr(column string, delta int64) clause.Expr {
	return gorm.Expr(
		fmt.Sprintf("CASE WHEN COALESCE(%s, 0) + ? < 0 THEN 0 ELSE COALESCE(%s, 0) + ? END", column, column),
		delta,
		delta,
	)
}

// AuthorIDByVideoID 读取视频作者的用户 ID；作者缺失时返回 ErrVideoAuthorNotFound。
//
// 与原 GetVideoAuthorID 的 DB 分支一致：先 JOIN 查，失败再按 author_username 二次查。
func (r *VideoRepository) AuthorIDByVideoID(ctx context.Context, videoID uint) (uint, error) {
	db := gormx.DB(ctx, r.db)

	var authorID uint
	if err := db.
		Table("videos v").
		Joins("JOIN users u ON u.username = v.author_username").
		Select("u.id").
		Where("v.id = ?", videoID).
		Scan(&authorID).Error; err != nil {
		return 0, err
	}

	if authorID != 0 {
		return authorID, nil
	}

	// 兼容异常数据：author_username 不存在时回退直接查询 video 关联
	var video po.Video
	if err := db.Select("id", "author_username").Take(&video, "id = ?", videoID).Error; err != nil {
		return 0, mapNotFound(err)
	}

	if video.AuthorUsername == "" {
		return 0, domain.ErrVideoAuthorNotFound
	}

	var user po.User
	if err := db.Select("id").Take(&user, "username = ?", video.AuthorUsername).Error; err != nil {
		return 0, mapNotFound(err)
	}

	if user.ID == 0 {
		return 0, domain.ErrVideoAuthorNotFound
	}

	return user.ID, nil
}

// PublishedAuthorIDByVideoID 读取已发布视频作者的用户 ID（供事务内互动流程使用）。
func (r *VideoRepository) PublishedAuthorIDByVideoID(ctx context.Context, videoID uint) (uint, error) {
	var authorID uint
	if err := gormx.DB(ctx, r.db).
		Table("videos v").
		Joins("JOIN users u ON u.username = v.author_username").
		Select("u.id").
		Where("v.id = ? AND v.status = ?", videoID, enum.VideoStatusPublished).
		Scan(&authorID).Error; err != nil {
		return 0, err
	}

	if authorID == 0 {
		return 0, domain.ErrVideoAuthorNotFound
	}

	return authorID, nil
}

// AuthorUsernameByVideoID 读取视频的作者用户名；视频不存在时返回 ErrRecordNotFound。
func (r *VideoRepository) AuthorUsernameByVideoID(ctx context.Context, videoID uint) (string, error) {
	var row po.Video
	if err := gormx.DB(ctx, r.db).
		Select("id", "author_username").
		Where("id = ?", videoID).
		Take(&row).Error; err != nil {
		return "", mapNotFound(err)
	}

	return row.AuthorUsername, nil
}

// UsernameByUserID 读取用户名（读侧投影，允许直读 users 表，见 decisions.md D-011）。
func (r *VideoRepository) UsernameByUserID(ctx context.Context, userID uint) (string, error) {
	var username string
	if err := gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("id = ?", userID).
		Pluck("username", &username).Error; err != nil {
		return "", err
	}

	return username, nil
}

// UserIDByUsername 读取用户 ID 与规范化后的用户名（读侧投影）。
//
// 与原 fetchUserByUsername 一致：查询失败时返回 (0, 入参用户名)。
func (r *VideoRepository) UserIDByUsername(ctx context.Context, username string) (uint, string, error) {
	if username == "" {
		return 0, "", nil
	}

	var user po.User
	if err := gormx.DB(ctx, r.db).
		Select("id", "username").
		Where("username = ?", username).
		First(&user).Error; err != nil {
		return 0, username, nil
	}

	return user.ID, user.Username, nil
}

// LockUsers 按给定顺序对 users 行加写锁（要求在事务内调用）。
func (r *VideoRepository) LockUsers(ctx context.Context, userIDs []uint) error {
	db := gormx.DB(ctx, r.db)

	for _, id := range userIDs {
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Take(&po.User{}, "id = ?", id).Error; err != nil {
			return mapNotFound(err)
		}
	}

	return nil
}

// UpdateStatus 无条件更新视频状态。
func (r *VideoRepository) UpdateStatus(ctx context.Context, videoID uint, status enum.VideoStatus) error {
	return gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Where("id = ?", videoID).
		Update("status", status).Error
}

// UpdateStatusFrom 仅当当前状态等于 from 时更新为 to，返回影响行数。
func (r *VideoRepository) UpdateStatusFrom(
	ctx context.Context,
	videoID uint,
	from, to enum.VideoStatus,
) (int64, error) {
	result := gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Where("id = ? AND status = ?", videoID, from).
		Update("status", to)

	return result.RowsAffected, result.Error
}

// UpdateStatusBatch 批量更新视频状态，返回影响行数。
func (r *VideoRepository) UpdateStatusBatch(
	ctx context.Context,
	videoIDs []uint,
	status enum.VideoStatus,
) (int64, error) {
	result := gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Where("id IN ?", videoIDs).
		Update("status", status)

	return result.RowsAffected, result.Error
}

// UpdateStatusBatchFrom 批量条件更新视频状态，返回影响行数。
func (r *VideoRepository) UpdateStatusBatchFrom(
	ctx context.Context,
	videoIDs []uint,
	from, to enum.VideoStatus,
) (int64, error) {
	result := gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Where("id IN ? AND status = ?", videoIDs, from).
		Update("status", to)

	return result.RowsAffected, result.Error
}

// IDsByStatus 过滤出指定状态的视频 ID。
func (r *VideoRepository) IDsByStatus(
	ctx context.Context,
	videoIDs []uint,
	status enum.VideoStatus,
) ([]uint, error) {
	var ids []uint
	if err := gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Where("id IN ? AND status = ?", videoIDs, status).
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}

	return ids, nil
}

// IDsByAuthor 读取该作者名下的全部视频 ID。
func (r *VideoRepository) IDsByAuthor(ctx context.Context, authorUsername string) ([]uint, error) {
	if authorUsername == "" {
		return nil, nil
	}

	var videoIDs []uint
	if err := gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Where("author_username = ?", authorUsername).
		Pluck("id", &videoIDs).Error; err != nil {
		return nil, err
	}

	return videoIDs, nil
}

// IDsByPartition 列出某分区下的全部视频 ID。
func (r *VideoRepository) IDsByPartition(ctx context.Context, partitionID uint) ([]uint, error) {
	var ids []uint
	if err := gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Where("partition_id = ?", partitionID).
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}

	return ids, nil
}

// CountByPartition 统计某分区下的视频数量。
func (r *VideoRepository) CountByPartition(ctx context.Context, partitionID uint) (int64, error) {
	var count int64
	if err := gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Where("partition_id = ?", partitionID).
		Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// UpdateFields 更新视频的部分字段（列名 → 值）。
func (r *VideoRepository) UpdateFields(ctx context.Context, videoID uint, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}

	return gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Where("id = ?", videoID).
		Updates(updates).Error
}

// ReplaceTags 覆盖视频的标签关联（tags 为空表示清空关联）。
func (r *VideoRepository) ReplaceTags(ctx context.Context, videoID uint, tags []domain.Tag) error {
	target := &po.Video{Model: po.Model{ID: videoID}}
	rows := toPOTags(tags)

	association := gormx.DB(ctx, r.db).Model(target).Association("Tags")
	if len(rows) == 0 {
		return association.Clear()
	}

	values := make([]any, 0, len(rows))
	for i := range rows {
		values = append(values, &rows[i])
	}

	return association.Replace(values...)
}

// TagsByVideoID 读取视频当前关联的标签。
func (r *VideoRepository) TagsByVideoID(ctx context.Context, videoID uint) ([]domain.Tag, error) {
	var rows []po.Tag
	if err := gormx.DB(ctx, r.db).
		Model(&po.Video{Model: po.Model{ID: videoID}}).
		Association("Tags").
		Find(&rows); err != nil {
		return nil, err
	}

	return toDomainTags(rows), nil
}

// FindTagsByIDs 按 ID 批量读取标签（无匹配时返回非 nil 空切片，与 GORM Find 语义一致）。
func (r *VideoRepository) FindTagsByIDs(ctx context.Context, tagIDs []uint) ([]domain.Tag, error) {
	if len(tagIDs) == 0 {
		return []domain.Tag{}, nil
	}

	var rows []po.Tag
	if err := gormx.DB(ctx, r.db).Find(&rows, "id IN ?", tagIDs).Error; err != nil {
		return nil, err
	}

	return toDomainTags(rows), nil
}

// CreateOrGetTag 创建标签（若已存在同名标签则返回现有）。
func (r *VideoRepository) CreateOrGetTag(ctx context.Context, name string) (*domain.Tag, error) {
	normalized, err := domain.ValidateTagName(name)
	if err != nil {
		return nil, err
	}

	db := gormx.DB(ctx, r.db)

	var row po.Tag
	if findErr := db.Where("name = ?", normalized).First(&row).Error; findErr == nil && row.ID > 0 {
		return toDomainTag(&row), nil
	}

	row = po.Tag{Name: normalized}
	if createErr := db.Create(&row).Error; createErr != nil {
		return nil, createErr
	}

	return toDomainTag(&row), nil
}

// SoftDeleteWithComments 作者软删视频：置为已删除并清理评论与评论点赞，返回删除计数。
//
// 与原 delete_service.go 的事务内三步一致（改状态 → 删评论点赞 → 删评论）。
// 应用层默认走 CommentPurger 端口（评论归 comment 上下文），本方法是领域接口声明的
// 完整实现，供不经跨上下文端口的调用路径使用。
func (r *VideoRepository) SoftDeleteWithComments(
	ctx context.Context,
	videoID uint,
) (comments, commentLikes int64, err error) {
	db := gormx.DB(ctx, r.db)

	if updateErr := db.Model(&po.Video{}).
		Where("id = ?", videoID).
		Update("status", enum.VideoStatusDeleted).Error; updateErr != nil {
		return 0, 0, updateErr
	}

	var commentIDs []uint
	if pluckErr := db.Model(&po.Comment{}).
		Where("video_id = ?", videoID).
		Pluck("id", &commentIDs).Error; pluckErr != nil {
		return 0, 0, pluckErr
	}

	if len(commentIDs) > 0 {
		likeResult := db.Where("comment_id IN ?", commentIDs).Delete(&po.CommentLike{})
		if likeResult.Error != nil {
			return 0, 0, likeResult.Error
		}

		commentLikes = likeResult.RowsAffected
	}

	commentResult := db.Where("video_id = ?", videoID).Delete(&po.Comment{})
	if commentResult.Error != nil {
		return 0, commentLikes, commentResult.Error
	}

	return commentResult.RowsAffected, commentLikes, nil
}

// StoragePathStillReferenced 判断对象路径是否仍被其它视频/资源引用。
func (r *VideoRepository) StoragePathStillReferenced(ctx context.Context, storedPath string) (bool, error) {
	db := gormx.DB(ctx, r.db)

	var count int64
	if err := db.Model(&po.Video{}).Where("cover = ?", storedPath).Count(&count).Error; err != nil {
		return false, err
	}

	if count > 0 {
		return true, nil
	}

	if err := db.Model(&po.VideoResource{}).Where("file_url = ?", storedPath).Count(&count).Error; err != nil {
		return false, err
	}

	if count > 0 {
		return true, nil
	}

	if err := db.Model(&po.VideoPartResource{}).Where("file_url = ?", storedPath).Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

// mapNotFound 把 GORM 的「记录不存在」映射为领域哨兵错误。
func mapNotFound(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrRecordNotFound
	}

	return err
}
