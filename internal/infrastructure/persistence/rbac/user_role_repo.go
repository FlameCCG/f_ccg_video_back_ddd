package rbac

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserRoleRepository 用户-角色绑定仓储的 GORM 实现。
//
// 读侧按 docs/architecture.md 第 4 节 直读 identity 的 users 表（仅存在性与 current_role_id）；
// 写侧只操作 rbac 自己的 user_roles 关联表。
//
// 与原实现的差异：原代码用 GORM 的 Association("Roles").Append/Replace，
// 其内部会对 users 行执行一次 Save（触发 po.User 的 BeforeSave 钩子）。
// 这里改为直接写 user_roles 关联表，落库结果完全相同，
// 但不会因为「只带主键的 User 结构体」把 open_id 误规范化为 NULL。
type UserRoleRepository struct {
	db *gorm.DB
}

// 编译期断言：满足领域侧仓储接口。
var _ domain.UserRoleRepository = (*UserRoleRepository)(nil)

// NewUserRoleRepository 构造用户-角色绑定仓储。
func NewUserRoleRepository(db *gorm.DB) *UserRoleRepository {
	return &UserRoleRepository{db: db}
}

// UserExists 判断用户是否存在。
func (r *UserRoleRepository) UserExists(ctx context.Context, userID uint) (bool, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).Select("id").First(&row, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// CurrentRoleID 读取用户当前角色 ID；用户不存在时 found 为 false。
func (r *UserRoleRepository) CurrentRoleID(ctx context.Context, userID uint) (uint, bool, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).
		Select("id", "current_role_id").
		First(&row, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, false, nil
		}

		return 0, false, err
	}

	return row.CurrentRoleID, true, nil
}

// RolesByUser 读取用户已绑定的角色；用户不存在时返回 domain.ErrUserNotFound。
func (r *UserRoleRepository) RolesByUser(ctx context.Context, userID uint) ([]*domain.Role, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).Preload("Roles").First(&row, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}

		return nil, err
	}

	return toDomainRolePtrs(row.Roles), nil
}

// AppendRoles 为用户追加角色绑定（已存在的绑定忽略）。
func (r *UserRoleRepository) AppendRoles(ctx context.Context, userID uint, roles []*domain.Role) error {
	rows := toPOUserRoles(userID, roles)
	if len(rows) == 0 {
		return nil
	}

	return gormx.DB(ctx, r.db).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&rows).Error
}

// RemoveRoles 移除用户的角色绑定。
func (r *UserRoleRepository) RemoveRoles(ctx context.Context, userID uint, roles []*domain.Role) error {
	roleIDs := roleIDsOf(roles)
	if len(roleIDs) == 0 {
		return nil
	}

	return gormx.DB(ctx, r.db).
		Where("user_id = ? AND role_id IN ?", userID, roleIDs).
		Delete(&po.UserRole{}).Error
}

// ReplaceRoles 用给定集合整体替换用户的角色绑定。
func (r *UserRoleRepository) ReplaceRoles(ctx context.Context, userID uint, roles []*domain.Role) error {
	db := gormx.DB(ctx, r.db)
	roleIDs := roleIDsOf(roles)

	if len(roleIDs) == 0 {
		return db.Where("user_id = ?", userID).Delete(&po.UserRole{}).Error
	}

	if err := db.Where("user_id = ? AND role_id NOT IN ?", userID, roleIDs).
		Delete(&po.UserRole{}).Error; err != nil {
		return err
	}

	rows := toPOUserRoles(userID, roles)

	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

// toPOUserRoles 构造用户-角色关联行（跳过空角色与零值 ID）。
func toPOUserRoles(userID uint, roles []*domain.Role) []po.UserRole {
	rows := make([]po.UserRole, 0, len(roles))

	for _, role := range roles {
		if role == nil || role.ID == 0 {
			continue
		}

		rows = append(rows, po.UserRole{UserID: userID, RoleID: role.ID})
	}

	return rows
}

// roleIDsOf 提取角色主键列表（跳过空角色与零值 ID）。
func roleIDsOf(roles []*domain.Role) []uint {
	ids := make([]uint, 0, len(roles))

	for _, role := range roles {
		if role == nil || role.ID == 0 {
			continue
		}

		ids = append(ids, role.ID)
	}

	return ids
}
