package po

// UserRole 用户角色关联表.
type UserRole struct {
	UserID uint `gorm:"primaryKey;comment:用户ID" json:"userId"`
	RoleID uint `gorm:"primaryKey;comment:角色ID" json:"roleId"`
	User   User `gorm:"foreignKey:UserID" json:"-"`
	Role   Role `gorm:"foreignKey:RoleID" json:"-"`
}

// TableName 显式绑定表名 user_roles，保持迁移与仓储 SQL 使用同一张表。
func (UserRole) TableName() string {
	return "user_roles"
}
