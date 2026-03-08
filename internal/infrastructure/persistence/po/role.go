package po

// Role 角色表.
type Role struct {
	Model
	Name  string  `gorm:"size:50;not null;uniqueIndex;comment:角色名称" json:"name"`
	Desc  string  `gorm:"size:200;comment:角色描述" json:"desc"`
	Users []*User `gorm:"many2many:user_roles;" json:"users,omitempty"` // 多对多：角色-用户
	Menus []*Menu `gorm:"many2many:role_menus;" json:"menus,omitempty"` // 多对多：角色-菜单
}

// TableName 显式绑定表名 roles，保持迁移与仓储 SQL 使用同一张表。
func (Role) TableName() string {
	return "roles"
}
