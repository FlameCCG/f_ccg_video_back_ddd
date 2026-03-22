package rbac

import (
	"context"
	"errors"

	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"
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
var _ rbacdomain.ApiResourceRepository = (*ApiResourceRepository)(nil)

// NewApiResourceRepository 构造接口资源仓储。
func NewApiResourceRepository(db *gorm.DB) *ApiResourceRepository {
	return &ApiResourceRepository{db: db}
}

// ListOrdered 按 path、method 升序读取全部资源。
func (r *ApiResourceRepository) ListOrdered(ctx context.Context) ([]*rbacdomain.ApiResource, error) {
	var rows []po.ApiResource
	if err := gormx.DB(ctx, r.db).Order("path ASC, method ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	return toDomainApiResources(rows), nil
}

// ListPathMethod 只读 path、method（权限覆盖计算用）。
func (r *ApiResourceRepository) ListPathMethod(ctx context.Context) ([]*rbacdomain.ApiResource, error) {
	var rows []po.ApiResource
	if err := gormx.DB(ctx, r.db).Select("path", "method").Find(&rows).Error; err != nil {
		return nil, err
	}

	return toDomainApiResources(rows), nil
}

// ListAll 读取全部资源（同步比对用）。
func (r *ApiResourceRepository) ListAll(ctx context.Context) ([]*rbacdomain.ApiResource, error) {
	var rows []po.ApiResource
	if err := gormx.DB(ctx, r.db).Find(&rows).Error; err != nil {
		return nil, err
	}

	return toDomainApiResources(rows), nil
}

// Create 创建资源并返回持久化结果，不修改输入。
func (r *ApiResourceRepository) Create(ctx context.Context, resource *rbacdomain.ApiResource) (*rbacdomain.ApiResource, error) {
	row := toPOApiResource(resource)
	if err := gormx.DB(ctx, r.db).Create(&row).Error; err != nil {
		return nil, err
	}

	return toDomainApiResource(&row), nil
}

// UpdateMetadata 保留字段级更新及旧 tags JSON 的比较规则。
func (r *ApiResourceRepository) UpdateMetadata(
	ctx context.Context,
	resourceID uint,
	change rbacdomain.ResourceMetadataChange,
) (bool, error) {
	var current po.ApiResource
	db := gormx.DB(ctx, r.db)
	if err := db.Select("id", "tags").Take(&current, "id = ?", resourceID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	updates := make(map[string]any, 2)

	if change.Summary != nil {
		updates["summary"] = *change.Summary
	}

	tags := (resourceTags{}).encode(change.Tags)
	if !(resourceTags{}).equal(current.Tags, tags) {
		updates["tags"] = datatypes.JSON(tags)
	}

	if len(updates) == 0 {
		return false, nil
	}

	err := db.Model(&po.ApiResource{}).
		Where("id = ?", resourceID).
		Updates(updates).Error
	return err == nil, err
}

// DeleteByID 删除资源。
func (r *ApiResourceRepository) DeleteByID(ctx context.Context, resourceID uint) error {
	return gormx.DB(ctx, r.db).Delete(&po.ApiResource{}, resourceID).Error
}
