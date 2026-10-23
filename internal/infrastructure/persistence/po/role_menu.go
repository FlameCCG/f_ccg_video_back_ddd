package po

// RoleMenu 角色菜单关联表.
type RoleMenu struct {
	RoleID      uint `gorm:"primaryKey;comment:角色ID" json:"roleId"`
	MenuID      uint `gorm:"primaryKey;comment:菜单ID" json:"menuId"`
	IsInherited bool `gorm:"default:false;comment:是否继承" json:"isInherited"`
}

// TableName 显式绑定表名 role_menus，保持迁移与仓储 SQL 使用同一张表。
func (RoleMenu) TableName() string {
	return "role_menus"
}
