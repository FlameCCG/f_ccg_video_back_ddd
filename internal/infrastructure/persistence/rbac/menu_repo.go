package rbac

import (
	"context"
	"errors"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// MenuRepository 菜单仓储的 GORM 实现。
type MenuRepository struct {
	db *gorm.DB
}

// 编译期断言：满足领域侧仓储接口。
var _ domain.MenuRepository = (*MenuRepository)(nil)

// NewMenuRepository 构造菜单仓储。
func NewMenuRepository(db *gorm.DB) *MenuRepository {
	return &MenuRepository{db: db}
}

// FindByID 按主键读取菜单；不存在时返回 (nil, nil)。
func (r *MenuRepository) FindByID(ctx context.Context, menuID uint) (*domain.Menu, error) {
	var row po.Menu
	if err := gormx.DB(ctx, r.db).First(&row, menuID).Error; err != nil {
		return nil, ignoreNotFound(err)
	}

	return toDomainMenu(&row), nil
}

// FindByTitle 按标题读取菜单；不存在时返回 (nil, nil)。
func (r *MenuRepository) FindByTitle(ctx context.Context, title string) (*domain.Menu, error) {
	var row po.Menu
	if err := gormx.DB(ctx, r.db).Take(&row, "title = ?", title).Error; err != nil {
		return nil, ignoreNotFound(err)
	}

	return toDomainMenu(&row), nil
}

// ExistsByID 判断菜单是否存在。
func (r *MenuRepository) ExistsByID(ctx context.Context, menuID uint) (bool, error) {
	var row po.Menu
	if err := gormx.DB(ctx, r.db).Select("id").Take(&row, "id = ?", menuID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// FindByIDs 批量读取菜单。
func (r *MenuRepository) FindByIDs(ctx context.Context, menuIDs []uint) ([]*domain.Menu, error) {
	var rows []po.Menu
	if err := gormx.DB(ctx, r.db).Where("id IN ?", menuIDs).Find(&rows).Error; err != nil {
		return nil, err
	}

	return toDomainMenuValues(rows), nil
}

// ListOrdered 按 sort_order 升序读取全部菜单。
func (r *MenuRepository) ListOrdered(ctx context.Context) ([]*domain.Menu, error) {
	var rows []*po.Menu
	if err := gormx.DB(ctx, r.db).Order("sort_order ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	return toDomainMenus(rows), nil
}

// ListByRole 读取角色的全部菜单（不区分是否继承），按 sort_order 升序去重。
func (r *MenuRepository) ListByRole(ctx context.Context, roleID uint) ([]*domain.Menu, error) {
	var rows []*po.Menu

	err := gormx.DB(ctx, r.db).Table("menus").
		Joins("JOIN role_menus ON menus.id = role_menus.menu_id").
		Where("role_menus.role_id = ?", roleID).
		Order("menus.sort_order ASC").
		Distinct().
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	return toDomainMenus(rows), nil
}

// ListByRoleInherited 读取角色的直连或继承菜单，按 sort_order 升序去重。
func (r *MenuRepository) ListByRoleInherited(ctx context.Context, roleID uint, inherited bool) ([]*domain.Menu, error) {
	var rows []*po.Menu

	err := gormx.DB(ctx, r.db).Table("menus").
		Joins("JOIN role_menus ON menus.id = role_menus.menu_id").
		Where("role_menus.role_id = ? AND role_menus.is_inherited = ?", roleID, inherited).
		Order("menus.sort_order ASC").
		Distinct().
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	return toDomainMenus(rows), nil
}

// CountChildren 统计子菜单数量。
func (r *MenuRepository) CountChildren(ctx context.Context, parentID uint) (int64, error) {
	var count int64
	if err := gormx.DB(ctx, r.db).Model(&po.Menu{}).
		Where("parent_id = ?", parentID).
		Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// CountByTitleExcluding 统计同标题且排除指定主键的菜单数量。
func (r *MenuRepository) CountByTitleExcluding(ctx context.Context, title string, excludeID uint) (int64, error) {
	var count int64
	if err := gormx.DB(ctx, r.db).Model(&po.Menu{}).
		Where("title = ? AND id != ?", title, excludeID).
		Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// Create 创建菜单并回填自增 ID 与时间戳。
func (r *MenuRepository) Create(ctx context.Context, menu *domain.Menu) error {
	row := toPOMenu(menu)
	if err := gormx.DB(ctx, r.db).Create(&row).Error; err != nil {
		return err
	}

	menu.ID = row.ID
	menu.CreatedAt = row.CreatedAt
	menu.UpdatedAt = row.UpdatedAt

	return nil
}

// UpdateFields 按需更新菜单字段；update 为空时不产生 UPDATE。
func (r *MenuRepository) UpdateFields(ctx context.Context, menuID uint, update domain.MenuFieldUpdate) error {
	if update.IsEmpty() {
		return nil
	}

	updates := make(map[string]any, 10)

	if update.Title != nil {
		updates["title"] = *update.Title
	}

	if update.TitleEn != nil {
		updates["title_en"] = *update.TitleEn
	}

	if update.TitleJa != nil {
		updates["title_ja"] = *update.TitleJa
	}

	if update.Icon != nil {
		updates["icon"] = *update.Icon
	}

	if update.Path != nil {
		updates["path"] = *update.Path
	}

	if update.Name != nil {
		updates["name"] = *update.Name
	}

	if update.Component != nil {
		updates["component"] = *update.Component
	}

	if update.KeepAlive != nil {
		updates["keep_alive"] = *update.KeepAlive
	}

	switch {
	case update.ClearParent:
		updates["parent_id"] = nil
	case update.ParentID != nil:
		updates["parent_id"] = *update.ParentID
	}

	if update.SortOrder != nil {
		updates["sort_order"] = *update.SortOrder
	}

	return gormx.DB(ctx, r.db).Model(&po.Menu{Model: po.Model{ID: menuID}}).Updates(updates).Error
}

// DeleteByID 删除菜单。
func (r *MenuRepository) DeleteByID(ctx context.Context, menuID uint) error {
	return gormx.DB(ctx, r.db).Delete(&po.Menu{Model: po.Model{ID: menuID}}).Error
}
