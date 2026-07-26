package po

import "gorm.io/gorm"

// Role 角色表.
type Role struct {
	Model
	Name  string  `gorm:"size:50;not null;uniqueIndex;comment:角色名称" json:"name"`
	Desc  string  `gorm:"size:200;comment:角色描述" json:"desc"`
	Users []*User `gorm:"many2many:user_roles;" json:"users,omitempty"` // 多对多：角色-用户
	Menus []*Menu `gorm:"many2many:role_menus;" json:"menus,omitempty"` // 多对多：角色-菜单
}

// TableName 方法.
func (Role) TableName() string {
	return "roles"
}

// AfterDelete 删除角色后自动清理关联关系.
func (r Role) AfterDelete(db *gorm.DB) error {
	// 删除角色菜单关联
	if err := db.Where("role_id = ?", r.ID).Delete(&RoleMenu{}).Error; err != nil {
		return err
	}
	// 删除角色用户关联
	if err := db.Where("role_id = ?", r.ID).Delete(&UserRole{}).Error; err != nil {
		return err
	}

	return nil
}
