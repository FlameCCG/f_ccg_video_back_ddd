package po

// UserFocus 用户关注关系.
type UserFocus struct {
	Model
	UserID      uint `gorm:"not null;uniqueIndex:idx_user_focus;comment:用户ID" json:"userID"`
	User        User `gorm:"foreignKey:UserID" json:"-"`
	FocusUserID uint `gorm:"not null;uniqueIndex:idx_user_focus;comment:关注的用户ID" json:"focusID"`
	FocusUser   User `gorm:"foreignKey:FocusUserID" json:"-"`
}

// TableName 方法.
func (UserFocus) TableName() string {
	return "user_focus"
}
