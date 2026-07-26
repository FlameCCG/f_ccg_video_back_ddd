package po

// UserLoginRecord 用户登录 IP 记录.
type UserLoginRecord struct {
	Model
	UserID    uint   `gorm:"not null;index;comment:用户ID" json:"userId"`
	IP        string `gorm:"size:45;not null;index;comment:登录IP" json:"ip"`
	Addr      string `gorm:"size:200;comment:IP归属地" json:"addr"`
	UserAgent string `gorm:"size:255;comment:User-Agent" json:"userAgent"`
	LoginType string `gorm:"size:20;index;comment:登录方式(pwd/qq)" json:"loginType"`
}

// TableName 方法.
func (UserLoginRecord) TableName() string {
	return "user_login_records"
}
