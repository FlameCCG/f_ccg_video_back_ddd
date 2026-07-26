package contentops

import (
	"context"
	"errors"

	domain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// BannerRepository 轮播图仓储的 GORM 实现（原 service/banner_service/banner_service.go 的落库部分）。
//
// SQL 语义（列名、反引号转义的 `show`、排序、分页）与原实现逐字一致。
type BannerRepository struct {
	db *gorm.DB
}

// 编译期断言：满足领域侧仓储接口。
var _ domain.BannerRepository = (*BannerRepository)(nil)

// NewBannerRepository 构造轮播图仓储。
func NewBannerRepository(db *gorm.DB) *BannerRepository {
	return &BannerRepository{db: db}
}

// ListVisible 按类型与分区读取可展示的轮播图，id 倒序。
func (r *BannerRepository) ListVisible(
	ctx context.Context,
	bannerType enum.BannerType,
	partitionID *uint,
) ([]domain.Banner, error) {
	query := gormx.DB(ctx, r.db).Model(&po.Banner{}).
		Where("`show` = ?", true)
	if bannerType > 0 {
		query = query.Where("type = ?", bannerType)
	}

	if partitionID != nil {
		query = query.Where("partition_id = ?", *partitionID)
	}

	var list []po.Banner
	if err := query.Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}

	return toDomainBanners(list), nil
}

// ListAdmin 后台分页列表：先取总数，总数为 0 时直接返回空列表。
func (r *BannerRepository) ListAdmin(
	ctx context.Context,
	q domain.BannerAdminQuery,
) ([]domain.Banner, int64, error) {
	query := gormx.DB(ctx, r.db).Model(&po.Banner{})
	if q.Type > 0 {
		query = query.Where("type = ?", q.Type)
	}

	if q.PartitionID != nil {
		query = query.Where("partition_id = ?", *q.PartitionID)
	}

	if q.Show != nil {
		query = query.Where("`show` = ?", *q.Show)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []domain.Banner{}, 0, nil
	}

	var list []po.Banner
	if err := query.Order("id DESC").
		Offset((q.Page - 1) * q.PageSize).
		Limit(q.PageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return toDomainBanners(list), total, nil
}

// ListVisibleByIDs 按给定 ID 顺序读取可展示的轮播图，缺失的 ID 直接跳过。
func (r *BannerRepository) ListVisibleByIDs(
	ctx context.Context,
	bannerType enum.BannerType,
	ids []uint,
) ([]domain.Banner, error) {
	if len(ids) == 0 {
		return []domain.Banner{}, nil
	}

	query := gormx.DB(ctx, r.db).
		Model(&po.Banner{}).
		Where("`show` = ?", true).
		Where("id IN ?", ids)
	if bannerType > 0 {
		query = query.Where("type = ?", bannerType)
	}

	list := make([]po.Banner, 0, len(ids))
	if err := query.Find(&list).Error; err != nil {
		return nil, err
	}

	bannerMap := make(map[uint]domain.Banner, len(list))
	for i := range list {
		bannerMap[list[i].ID] = *toDomainBanner(&list[i])
	}

	ordered := make([]domain.Banner, 0, len(list))

	for _, id := range ids {
		banner, ok := bannerMap[id]
		if !ok {
			continue
		}

		ordered = append(ordered, banner)
	}

	return ordered, nil
}

// FindByID 按主键读取轮播图；不存在时返回 (nil, nil)。
func (r *BannerRepository) FindByID(ctx context.Context, bannerID uint) (*domain.Banner, error) {
	var row po.Banner
	if err := gormx.DB(ctx, r.db).First(&row, bannerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomainBanner(&row), nil
}

// Create 新增轮播图，并把自增主键与时间戳回填到入参实体。
func (r *BannerRepository) Create(ctx context.Context, banner *domain.Banner) error {
	if banner == nil {
		return domain.ErrBannerInvalidParam
	}

	row := toPOBanner(banner)
	if err := gormx.DB(ctx, r.db).Create(row).Error; err != nil {
		return err
	}

	banner.ID = row.ID
	banner.CreatedAt = row.CreatedAt
	banner.UpdatedAt = row.UpdatedAt

	return nil
}

// Update 按主键部分更新轮播图字段。
func (r *BannerRepository) Update(ctx context.Context, bannerID uint, changes domain.BannerChanges) error {
	if bannerID == 0 {
		return domain.ErrBannerInvalidParam
	}

	updates := toPOBannerChanges(changes)
	if len(updates) == 0 {
		return domain.ErrBannerNoUpdatableField
	}

	return gormx.DB(ctx, r.db).
		Model(&po.Banner{}).
		Where("id = ?", bannerID).
		Updates(updates).Error
}

// Delete 按主键批量删除轮播图。
func (r *BannerRepository) Delete(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	return gormx.DB(ctx, r.db).
		Where("id IN ?", ids).
		Delete(&po.Banner{}).Error
}
