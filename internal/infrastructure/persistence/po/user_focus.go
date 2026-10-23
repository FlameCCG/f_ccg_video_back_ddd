package po

// UserFocus 用户关注关系.
type UserFocus struct {
	Model
	UserID      uint `gorm:"not null;uniqueIndex:idx_user_focus;comment:用户ID" json:"userID"`
	User        User `gorm:"foreignKey:UserID" json:"-"`
	FocusUserID uint `gorm:"not null;uniqueIndex:idx_user_focus;comment:关注的用户ID" json:"focusID"`
	FocusUser   User `gorm:"foreignKey:FocusUserID" json:"-"`
}

// TableName 显式绑定表名 user_focus，保持迁移与仓储 SQL 使用同一张表。
func (UserFocus) TableName() string {
	return "user_focus"
}
