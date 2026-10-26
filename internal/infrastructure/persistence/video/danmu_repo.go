package video

import (
	"context"
	"errors"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// DanmuRepository 弹幕写侧仓储的 GORM 实现。
type DanmuRepository struct {
	db *gorm.DB
}

var _ domain.DanmuRepository = (*DanmuRepository)(nil)

// NewDanmuRepository 构造弹幕仓储。
func NewDanmuRepository(db *gorm.DB) *DanmuRepository {
	return &DanmuRepository{db: db}
}

// Create 写入弹幕并递增所属分P的弹幕数（单事务）。
func (r *DanmuRepository) Create(ctx context.Context, record *domain.Danmu) error {
	if record == nil {
		return domain.ErrParamInvalid
	}

	row := toPODanmu(record)

	if err := gormx.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		if createErr := tx.Create(row).Error; createErr != nil {
			return createErr
		}

		if row.VideoPartID != 0 {
			if updateErr := tx.Model(&po.Part{}).
				Where("id = ?", row.VideoPartID).
				UpdateColumn("danmu_count", gorm.Expr("danmu_count + 1")).Error; updateErr != nil {
				return updateErr
			}
		}

		return nil
	}); err != nil {
		return err
	}

	record.ID = row.ID
	record.CreatedAt = row.CreatedAt
	record.UpdatedAt = row.UpdatedAt

	return nil
}

// FindByID 读取弹幕（id/user_id/video_id/video_part_id）；不存在时返回 ErrDanmuNotFound。
func (r *DanmuRepository) FindByID(ctx context.Context, danmuID uint) (*domain.Danmu, error) {
	var row po.Danmu
	if err := gormx.DB(ctx, r.db).
		Select("id", "user_id", "video_id", "video_part_id").
		Take(&row, "id = ?", danmuID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrDanmuNotFound
		}

		return nil, err
	}

	return toDomainDanmu(&row), nil
}

// FindByIDs 批量读取弹幕（id/video_id/video_part_id）。
func (r *DanmuRepository) FindByIDs(ctx context.Context, danmuIDs []uint) ([]domain.Danmu, error) {
	if len(danmuIDs) == 0 {
		return nil, nil
	}

	var rows []po.Danmu
	if err := gormx.DB(ctx, r.db).
		Select("id", "video_id", "video_part_id").
		Find(&rows, "id IN ?", danmuIDs).Error; err != nil {
		return nil, err
	}

	return toDomainDanmus(rows), nil
}

// DeleteMany 删除弹幕及其点赞（单事务）。
func (r *DanmuRepository) DeleteMany(ctx context.Context, danmuIDs []uint) error {
	if len(danmuIDs) == 0 {
		return nil
	}

	return gormx.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("danmu_id IN ?", danmuIDs).Delete(&po.DanmuLike{}).Error; err != nil {
			return err
		}

		return tx.Where("id IN ?", danmuIDs).Delete(&po.Danmu{}).Error
	})
}

// DecrPartDanmuCount 扣减分P弹幕数（不小于 0）。
func (r *DanmuRepository) DecrPartDanmuCount(ctx context.Context, partID uint, delta int64) error {
	if partID == 0 || delta <= 0 {
		return nil
	}

	return gormx.DB(ctx, r.db).
		Model(&po.Part{}).
		Where("id = ?", partID).
		UpdateColumn("danmu_count", gorm.Expr("GREATEST(danmu_count - ?, 0)", delta)).Error
}

// PartAvailability 一次查询视频与分P可用性快照。
func (r *DanmuRepository) PartAvailability(
	ctx context.Context,
	videoID, partID uint,
) (domain.DanmuPartAvailability, error) {
	type availabilityRow struct {
		VideoExists int8  `gorm:"column:video_exists"`
		PartCount   int64 `gorm:"column:part_count"`
		OnlyPartID  uint  `gorm:"column:only_part_id"`
		PartExists  int8  `gorm:"column:part_exists"`
	}

	var row availabilityRow
	if err := gormx.DB(ctx, r.db).Raw(`
SELECT
  EXISTS(SELECT 1 FROM videos WHERE id = ? AND status = ?) AS video_exists,
  (SELECT COUNT(1) FROM video_parts WHERE video_id = ?) AS part_count,
  (SELECT id FROM video_parts WHERE video_id = ? ORDER BY sort_order ASC, id ASC LIMIT 1) AS only_part_id,
  EXISTS(SELECT 1 FROM video_parts WHERE id = ? AND video_id = ?) AS part_exists
`, videoID, enum.VideoStatusPublished, videoID, videoID, partID, videoID).Scan(&row).Error; err != nil {
		return domain.DanmuPartAvailability{}, err
	}

	return domain.DanmuPartAvailability{
		VideoExists: row.VideoExists > 0,
		PartCount:   row.PartCount,
		OnlyPartID:  row.OnlyPartID,
		PartExists:  row.PartExists > 0,
	}, nil
}

// ToggleLike 翻转弹幕点赞并维护 like_count；返回翻转后是否已点赞。
func (r *DanmuRepository) ToggleLike(ctx context.Context, danmuID, userID uint) (bool, error) {
	liked := false

	err := gormx.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		like := po.DanmuLike{DanmuID: danmuID, UserID: userID}

		createErr := tx.Create(&like).Error
		if createErr == nil {
			liked = true

			return tx.Model(&po.Danmu{}).
				Where("id = ?", danmuID).
				UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
		}

		if !gormx.IsDuplicateKeyError(createErr) {
			return createErr
		}

		del := tx.Where("danmu_id = ? AND user_id = ?", danmuID, userID).Delete(&po.DanmuLike{})
		if del.Error != nil {
			return del.Error
		}

		liked = false

		if del.RowsAffected == 0 {
			return nil
		}

		return tx.Model(&po.Danmu{}).
			Where("id = ?", danmuID).
			UpdateColumn("like_count", gorm.Expr("GREATEST(like_count - 1, 0)")).Error
	})
	if err != nil {
		return false, err
	}

	return liked, nil
}

// LikeCount 读取弹幕当前点赞数。
func (r *DanmuRepository) LikeCount(ctx context.Context, danmuID uint) (int64, error) {
	var likeCount int64
	if err := gormx.DB(ctx, r.db).
		Model(&po.Danmu{}).
		Select("like_count").
		Where("id = ?", danmuID).
		Scan(&likeCount).Error; err != nil {
		return 0, err
	}

	return likeCount, nil
}

// LikedSet 读取用户在给定弹幕集合上的点赞标记。
func (r *DanmuRepository) LikedSet(
	ctx context.Context,
	userID uint,
	danmuIDs []uint,
) (map[uint]struct{}, error) {
	if userID == 0 || len(danmuIDs) == 0 {
		return nil, nil
	}

	var likes []po.DanmuLike
	if err := gormx.DB(ctx, r.db).
		Select("danmu_id").
		Find(&likes, "user_id = ? AND danmu_id IN ?", userID, danmuIDs).Error; err != nil {
		return nil, err
	}

	set := make(map[uint]struct{}, len(likes))

	for _, like := range likes {
		if like.DanmuID != 0 {
			set[like.DanmuID] = struct{}{}
		}
	}

	return set, nil
}
