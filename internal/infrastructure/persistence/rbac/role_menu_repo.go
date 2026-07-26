package rbac

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// roleMenuTable 角色-菜单关联表名。
const roleMenuTable = "role_menus"

// RoleMenuRepository 角色-菜单关联仓储的 GORM 实现。
type RoleMenuRepository struct {
	db *gorm.DB
}

// 编译期断言：满足领域侧仓储接口。
var _ domain.RoleMenuRepository = (*RoleMenuRepository)(nil)

// NewRoleMenuRepository 构造角色-菜单关联仓储。
func NewRoleMenuRepository(db *gorm.DB) *RoleMenuRepository {
	return &RoleMenuRepository{db: db}
}

// InheritedMenuIDs 读取角色的继承菜单 ID 集合。
func (r *RoleMenuRepository) InheritedMenuIDs(ctx context.Context, roleID uint) (map[uint]struct{}, error) {
	var ids []uint
	if err := gormx.DB(ctx, r.db).
		Table(roleMenuTable).
		Select("menu_id").
		Where("role_id = ? AND is_inherited = ?", roleID, true).
		Pluck("menu_id", &ids).Error; err != nil {
		return nil, err
	}

	set := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}

	return set, nil
}

// MenuIDsByRole 读取角色的全部菜单 ID（不去重，不区分是否继承）。
func (r *RoleMenuRepository) MenuIDsByRole(ctx context.Context, roleID uint) ([]uint, error) {
	var ids []uint
	if err := gormx.DB(ctx, r.db).
		Table(roleMenuTable).
		Select("menu_id").
		Where("role_id = ?", roleID).
		Pluck("menu_id", &ids).Error; err != nil {
		return nil, err
	}

	return ids, nil
}

// DistinctMenuIDsByRole 读取角色的菜单 ID（去重）。
func (r *RoleMenuRepository) DistinctMenuIDsByRole(ctx context.Context, roleID uint) ([]uint, error) {
	var ids []uint
	if err := gormx.DB(ctx, r.db).
		Table(roleMenuTable).
		Select("DISTINCT menu_id").
		Where("role_id = ?", roleID).
		Pluck("menu_id", &ids).Error; err != nil {
		return nil, err
	}

	return ids, nil
}

// DistinctMenuIDsByRoles 读取多个角色的菜单 ID 并集（去重）。
func (r *RoleMenuRepository) DistinctMenuIDsByRoles(ctx context.Context, roleIDs []uint) ([]uint, error) {
	var ids []uint
	if err := gormx.DB(ctx, r.db).
		Table(roleMenuTable).
		Select("DISTINCT menu_id").
		Where("role_id IN ?", roleIDs).
		Pluck("menu_id", &ids).Error; err != nil {
		return nil, err
	}

	return ids, nil
}

// CreateIgnoreConflict 批量写入关联，冲突行忽略。
func (r *RoleMenuRepository) CreateIgnoreConflict(ctx context.Context, items []domain.RoleMenu) error {
	if len(items) == 0 {
		return nil
	}

	rows := toPORoleMenus(items)

	return gormx.DB(ctx, r.db).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&rows).Error
}

// EnsureRoleMenu 幂等确保存在一条 (roleID, menuID) 直连关联。
func (r *RoleMenuRepository) EnsureRoleMenu(ctx context.Context, roleID, menuID uint) error {
	row := po.RoleMenu{RoleID: roleID, MenuID: menuID}

	return gormx.DB(ctx, r.db).FirstOrCreate(&row, row).Error
}

// DeleteDirectByRole 删除角色的全部直连关联。
func (r *RoleMenuRepository) DeleteDirectByRole(ctx context.Context, roleID uint) error {
	return gormx.DB(ctx, r.db).
		Where("role_id = ? AND is_inherited = ?", roleID, false).
		Delete(&po.RoleMenu{}).Error
}

// DeleteDirectByRoleExcept 删除角色除 keepMenuIDs 之外的直连关联。
func (r *RoleMenuRepository) DeleteDirectByRoleExcept(ctx context.Context, roleID uint, keepMenuIDs []uint) error {
	return gormx.DB(ctx, r.db).
		Where("role_id = ? AND is_inherited = ? AND menu_id NOT IN ?", roleID, false, keepMenuIDs).
		Delete(&po.RoleMenu{}).Error
}

// DeleteDirectByRoleAndMenus 删除角色在 menuIDs 内的直连关联。
func (r *RoleMenuRepository) DeleteDirectByRoleAndMenus(ctx context.Context, roleID uint, menuIDs []uint) error {
	return gormx.DB(ctx, r.db).
		Where("role_id = ? AND is_inherited = ? AND menu_id IN ?", roleID, false, menuIDs).
		Delete(&po.RoleMenu{}).Error
}

// DeleteInheritedByRole 删除角色的全部继承关联。
func (r *RoleMenuRepository) DeleteInheritedByRole(ctx context.Context, roleID uint) error {
	return gormx.DB(ctx, r.db).
		Where("role_id = ? AND is_inherited = ?", roleID, true).
		Delete(&po.RoleMenu{}).Error
}

// DeleteInheritedByRoleExcept 删除角色除 keepMenuIDs 之外的继承关联。
func (r *RoleMenuRepository) DeleteInheritedByRoleExcept(ctx context.Context, roleID uint, keepMenuIDs []uint) error {
	return gormx.DB(ctx, r.db).
		Where("role_id = ? AND is_inherited = ? AND menu_id NOT IN ?", roleID, true, keepMenuIDs).
		Delete(&po.RoleMenu{}).Error
}

// DeleteByMenu 删除某个菜单的全部角色关联。
func (r *RoleMenuRepository) DeleteByMenu(ctx context.Context, menuID uint) error {
	return gormx.DB(ctx, r.db).
		Where("menu_id = ?", menuID).
		Delete(&po.RoleMenu{}).Error
}
