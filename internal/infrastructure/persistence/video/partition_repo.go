package video

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// PartitionRepository 视频分区仓储的 GORM 实现。
type PartitionRepository struct {
	db *gorm.DB
}

var _ domain.PartitionRepository = (*PartitionRepository)(nil)

// NewPartitionRepository 构造分区仓储。
func NewPartitionRepository(db *gorm.DB) *PartitionRepository {
	return &PartitionRepository{db: db}
}

// Create 创建分区并回填 ID。
func (r *PartitionRepository) Create(ctx context.Context, p *domain.Partition) error {
	if p == nil {
		return domain.ErrParamInvalid
	}

	row := toPOPartition(p)
	if err := gormx.DB(ctx, r.db).Create(row).Error; err != nil {
		return err
	}

	p.ID = row.ID

	return nil
}

// Update 覆盖更新分区字段。
//
// 与原实现一致：使用 Save 语义（Updates + 全字段），bool 字段的 false 也会写入。
func (r *PartitionRepository) Update(ctx context.Context, p *domain.Partition) error {
	if p == nil || p.ID == 0 {
		return domain.ErrParamInvalid
	}

	return gormx.DB(ctx, r.db).
		Model(&po.VideoPartition{}).
		Where("id = ?", p.ID).
		Updates(map[string]any{
			"name":           p.Name,
			"icon":           p.Icon,
			"sort_order":     p.SortOrder,
			"is_active":      p.IsActive,
			"is_submittable": p.IsSubmittable,
		}).Error
}

// Delete 删除分区，返回影响行数。
func (r *PartitionRepository) Delete(ctx context.Context, id uint) (int64, error) {
	result := gormx.DB(ctx, r.db).Delete(&po.VideoPartition{}, "id = ?", id)

	return result.RowsAffected, result.Error
}

// FindByID 读取分区；不存在时返回 ErrRecordNotFound。
func (r *PartitionRepository) FindByID(ctx context.Context, id uint) (*domain.Partition, error) {
	var row po.VideoPartition
	if err := gormx.DB(ctx, r.db).Take(&row, "id = ?", id).Error; err != nil {
		return nil, mapNotFound(err)
	}

	return toDomainPartition(&row), nil
}

// List 分页列出分区。
func (r *PartitionRepository) List(ctx context.Context, page, pageSize int) ([]domain.Partition, int64, error) {
	query := gormx.DB(ctx, r.db).Model(&po.VideoPartition{})

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []domain.Partition{}, 0, nil
	}

	var rows []po.VideoPartition
	if err := query.Session(&gorm.Session{}).
		Order("sort_order ASC, id ASC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	return toDomainPartitions(rows), total, nil
}

// ListSubmittable 列出启用且允许投稿的分区。
func (r *PartitionRepository) ListSubmittable(ctx context.Context) ([]domain.Partition, error) {
	var rows []po.VideoPartition
	if err := gormx.DB(ctx, r.db).
		Model(&po.VideoPartition{}).
		Where("is_active = ? AND is_submittable = ?", true, true).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	return toDomainPartitions(rows), nil
}
