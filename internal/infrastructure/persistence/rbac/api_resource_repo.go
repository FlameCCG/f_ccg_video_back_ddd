package rbac

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ApiResourceRepository 接口资源仓储的 GORM 实现。
type ApiResourceRepository struct {
	db *gorm.DB
}

// 编译期断言：满足领域侧仓储接口。
var _ domain.ApiResourceRepository = (*ApiResourceRepository)(nil)

// NewApiResourceRepository 构造接口资源仓储。
func NewApiResourceRepository(db *gorm.DB) *ApiResourceRepository {
	return &ApiResourceRepository{db: db}
}

// ListOrdered 按 path、method 升序读取全部资源。
func (r *ApiResourceRepository) ListOrdered(ctx context.Context) ([]*domain.ApiResource, error) {
	var rows []po.ApiResource
	if err := gormx.DB(ctx, r.db).Order("path ASC, method ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	return toDomainApiResources(rows), nil
}

// ListPathMethod 只读 path、method（权限覆盖计算用）。
func (r *ApiResourceRepository) ListPathMethod(ctx context.Context) ([]*domain.ApiResource, error) {
	var rows []po.ApiResource
	if err := gormx.DB(ctx, r.db).Select("path", "method").Find(&rows).Error; err != nil {
		return nil, err
	}

	return toDomainApiResources(rows), nil
}

// ListAll 读取全部资源（同步比对用）。
func (r *ApiResourceRepository) ListAll(ctx context.Context) ([]*domain.ApiResource, error) {
	var rows []po.ApiResource
	if err := gormx.DB(ctx, r.db).Find(&rows).Error; err != nil {
		return nil, err
	}

	return toDomainApiResources(rows), nil
}

// Create 创建资源并回填自增 ID 与时间戳。
func (r *ApiResourceRepository) Create(ctx context.Context, resource *domain.ApiResource) error {
	row := toPOApiResource(resource)
	if err := gormx.DB(ctx, r.db).Create(&row).Error; err != nil {
		return err
	}

	resource.ID = row.ID
	resource.CreatedAt = row.CreatedAt
	resource.UpdatedAt = row.UpdatedAt

	return nil
}

// UpdateSummaryAndTags 更新资源说明与标签；两者均为 nil 时不产生 UPDATE。
func (r *ApiResourceRepository) UpdateSummaryAndTags(
	ctx context.Context,
	resourceID uint,
	summary *string,
	tags *[]byte,
) error {
	updates := make(map[string]any, 2)

	if summary != nil {
		updates["summary"] = *summary
	}

	if tags != nil {
		updates["tags"] = datatypes.JSON(*tags)
	}

	if len(updates) == 0 {
		return nil
	}

	return gormx.DB(ctx, r.db).Model(&po.ApiResource{}).
		Where("id = ?", resourceID).
		Updates(updates).Error
}

// DeleteByID 删除资源。
func (r *ApiResourceRepository) DeleteByID(ctx context.Context, resourceID uint) error {
	return gormx.DB(ctx, r.db).Delete(&po.ApiResource{}, resourceID).Error
}
