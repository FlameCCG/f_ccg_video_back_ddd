package po

// UserRole 用户角色关联表.
type UserRole struct {
	UserID uint `gorm:"primaryKey;comment:用户ID" json:"userId"`
	RoleID uint `gorm:"primaryKey;comment:角色ID" json:"roleId"`
	User   User `gorm:"foreignKey:UserID" json:"-"`
	Role   Role `gorm:"foreignKey:RoleID" json:"-"`
}

// TableName 方法.
func (UserRole) TableName() string {
	return "user_roles"
}
