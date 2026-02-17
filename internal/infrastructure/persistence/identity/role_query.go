package identity

import (
	"context"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// UserRoleQuery 提供用户角色的只读 GORM 投影。
//
// roles 与 user_roles 归 rbac 上下文所有；查询只执行角色校验，Casbin 绑定通过端口写入。
type UserRoleQuery struct {
	db *gorm.DB
}

var _ app.UserRoleQuery = (*UserRoleQuery)(nil)

// NewUserRoleQuery 构造用户角色查询。
func NewUserRoleQuery(db *gorm.DB) *UserRoleQuery {
	return &UserRoleQuery{db: db}
}

// RoleIDByName 按角色名读取角色 ID；不存在时返回错误。
func (q *UserRoleQuery) RoleIDByName(ctx context.Context, name string) (uint, error) {
	var row po.Role
	if err := gormx.DB(ctx, q.db).Where("name = ?", name).First(&row).Error; err != nil {
		return 0, err
	}

	return row.ID, nil
}

// HasRole 判断用户是否拥有指定角色。
func (q *UserRoleQuery) HasRole(ctx context.Context, userID, roleID uint) (bool, error) {
	var count int64
	if err := gormx.DB(ctx, q.db).
		Table("user_roles").
		Where("user_id = ? AND role_id = ?", userID, roleID).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

// LoadRoles 加载用户角色列表。
func (q *UserRoleQuery) LoadRoles(ctx context.Context, userID uint) ([]app.UserRoleItem, error) {
	var rows []app.UserRoleItem
	if err := gormx.DB(ctx, q.db).
		Table("user_roles").
		Select("roles.id AS id, roles.name AS name").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id = ?", userID).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	return rows, nil
}

// RoleMap 返回用户「角色 ID → 角色名」映射；无角色时返回 nil。
//
// 返回 nil（而非空 map）以保证响应中 roleMap 字段为 null，
func (q *UserRoleQuery) RoleMap(ctx context.Context, userID uint) (map[uint]string, error) {
	rows, err := q.LoadRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, nil
	}

	roleMap := make(map[uint]string, len(rows))
	for _, row := range rows {
		if _, exists := roleMap[row.ID]; !exists {
			roleMap[row.ID] = row.Name
		}
	}

	return roleMap, nil
}
