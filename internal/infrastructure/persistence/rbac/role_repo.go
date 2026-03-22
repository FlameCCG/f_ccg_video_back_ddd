package rbac

import (
	"context"
	"errors"

	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// RoleRepository 角色仓储的 GORM 实现。
type RoleRepository struct {
	db *gorm.DB
}

// 编译期断言：满足领域侧仓储接口。
var _ rbacdomain.RoleRepository = (*RoleRepository)(nil)

// NewRoleRepository 构造角色仓储。
func NewRoleRepository(db *gorm.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

// FindByID 按主键读取角色；不存在时返回 (nil, nil)。
func (r *RoleRepository) FindByID(ctx context.Context, roleID uint) (*rbacdomain.Role, error) {
	var row po.Role
	if err := gormx.DB(ctx, r.db).First(&row, roleID).Error; err != nil {
		return nil, ignoreNotFound(err)
	}

	return toDomainRole(&row), nil
}

// FindBriefByID 只读 id/name/desc；不存在时返回 (nil, nil)。
func (r *RoleRepository) FindBriefByID(ctx context.Context, roleID uint) (*rbacdomain.Role, error) {
	var row po.Role
	if err := gormx.DB(ctx, r.db).Select("id", "name", "desc").First(&row, roleID).Error; err != nil {
		return nil, ignoreNotFound(err)
	}

	return toDomainRole(&row), nil
}

// FindIDByID 只读主键，用于「角色是否存在」校验；不存在时返回 (nil, nil)。
func (r *RoleRepository) FindIDByID(ctx context.Context, roleID uint) (*rbacdomain.Role, error) {
	var row po.Role
	if err := gormx.DB(ctx, r.db).Select("id").First(&row, roleID).Error; err != nil {
		return nil, ignoreNotFound(err)
	}

	return toDomainRole(&row), nil
}

// FindByName 按角色名读取；不存在时返回 (nil, nil)。
func (r *RoleRepository) FindByName(ctx context.Context, name string) (*rbacdomain.Role, error) {
	var row po.Role
	if err := gormx.DB(ctx, r.db).Take(&row, "name = ?", name).Error; err != nil {
		return nil, ignoreNotFound(err)
	}

	return toDomainRole(&row), nil
}

// FindWithMenusByID 读取角色并预加载其菜单；不存在时返回 (nil, nil)。
func (r *RoleRepository) FindWithMenusByID(ctx context.Context, roleID uint) (*rbacdomain.Role, error) {
	var row po.Role
	if err := gormx.DB(ctx, r.db).Preload("Menus").First(&row, roleID).Error; err != nil {
		return nil, ignoreNotFound(err)
	}

	return toDomainRole(&row), nil
}

// FindByIDs 批量读取角色。
func (r *RoleRepository) FindByIDs(ctx context.Context, roleIDs []uint) ([]*rbacdomain.Role, error) {
	var rows []po.Role
	if err := gormx.DB(ctx, r.db).Where("id IN ?", roleIDs).Find(&rows).Error; err != nil {
		return nil, err
	}

	return toDomainRoles(rows), nil
}

// FindBriefByNames 按角色名批量读取 id/name。
func (r *RoleRepository) FindBriefByNames(ctx context.Context, names []string) ([]*rbacdomain.Role, error) {
	var rows []po.Role
	if err := gormx.DB(ctx, r.db).Select("id", "name").Where("name IN ?", names).Find(&rows).Error; err != nil {
		return nil, err
	}

	return toDomainRoles(rows), nil
}

// CountByName 统计同名角色数量。
func (r *RoleRepository) CountByName(ctx context.Context, name string) (int64, error) {
	var count int64
	if err := gormx.DB(ctx, r.db).Model(&po.Role{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// ListWithMenus 读取全部角色并预加载菜单。
func (r *RoleRepository) ListWithMenus(ctx context.Context) ([]*rbacdomain.Role, error) {
	var rows []po.Role
	if err := gormx.DB(ctx, r.db).Preload("Menus").Find(&rows).Error; err != nil {
		return nil, err
	}

	return toDomainRoles(rows), nil
}

// ListBrief 读取全部角色的 id/name/desc。
func (r *RoleRepository) ListBrief(ctx context.Context) ([]*rbacdomain.Role, error) {
	var rows []po.Role
	if err := gormx.DB(ctx, r.db).Select("id", "name", "desc").Find(&rows).Error; err != nil {
		return nil, err
	}

	return toDomainRoles(rows), nil
}

// UserCount 通过关联表 COUNT 统计仍绑定该角色的用户数，避免加载完整用户行。
func (r *RoleRepository) UserCount(ctx context.Context, roleID uint) (int64, error) {
	var count int64
	if err := gormx.DB(ctx, r.db).Model(&po.UserRole{}).
		Where("role_id = ?", roleID).
		Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// Create 创建角色并返回带自增 ID 与时间戳的角色。
func (r *RoleRepository) Create(ctx context.Context, role *rbacdomain.Role) (*rbacdomain.Role, error) {
	row := toPORole(role)
	if err := gormx.DB(ctx, r.db).Create(&row).Error; err != nil {
		return nil, err
	}

	snapshot := role.Snapshot()
	snapshot.ID = row.ID
	snapshot.CreatedAt = row.CreatedAt
	snapshot.UpdatedAt = row.UpdatedAt

	return rbacdomain.RestoreRole(snapshot), nil
}

// UpdateNameAndDesc 更新角色名与描述；两者均为 nil 时不产生 UPDATE。
func (r *RoleRepository) UpdateNameAndDesc(ctx context.Context, roleID uint, name, desc *string) error {
	updates := make(map[string]any, 2)

	if name != nil {
		updates["name"] = *name
	}

	if desc != nil {
		updates["desc"] = *desc
	}

	if len(updates) == 0 {
		return nil
	}

	return gormx.DB(ctx, r.db).Model(&po.Role{Model: po.Model{ID: roleID}}).Updates(updates).Error
}

// UpdateName 覆盖写角色名。
func (r *RoleRepository) UpdateName(ctx context.Context, roleID uint, name string) error {
	return gormx.DB(ctx, r.db).Model(&po.Role{Model: po.Model{ID: roleID}}).Update("name", name).Error
}

func (r *RoleRepository) DeleteByID(ctx context.Context, roleID uint) error {
	return gormx.DB(ctx, r.db).Delete(&po.Role{Model: po.Model{ID: roleID}}).Error
}

// AppendMenus 向角色追加直连菜单关联（role_menus.is_inherited 取默认值 false）。
func (r *RoleRepository) AppendMenus(ctx context.Context, role *rbacdomain.Role, menus []*rbacdomain.Menu) error {
	if role == nil || len(menus) == 0 {
		return nil
	}

	row := toPORole(role)
	rows := toPOMenus(menus)

	return gormx.DB(ctx, r.db).Model(&row).Association("Menus").Append(&rows)
}

// ignoreNotFound 把「记录不存在」折叠为 nil 错误，其余错误原样返回。
func ignoreNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}

	return err
}

// DeleteBindings is explicitly orchestrated with DeleteByID by the application transaction.
func (r *RoleRepository) DeleteBindings(ctx context.Context, roleID uint) error {
	db := gormx.DB(ctx, r.db)
	if err := db.Where("role_id = ?", roleID).Delete(&po.RoleMenu{}).Error; err != nil {
		return err
	}
	if err := db.Where("role_id = ?", roleID).Delete(&po.UserRole{}).Error; err != nil {
		return err
	}
	return db.Model(&po.User{}).Where("current_role_id = ?", roleID).Update("current_role_id", 0).Error
}
