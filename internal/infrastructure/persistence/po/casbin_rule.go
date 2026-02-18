package po

// CasbinRule 类型.
type CasbinRule struct {
	ID    uint   `gorm:"primarykey"`
	Ptype string `gorm:"type:varchar(100);"`
	V0    string `gorm:"type:varchar(100);"`
	V1    string `gorm:"type:varchar(100);"`
	V2    string `gorm:"type:varchar(100);"`
	V3    string `gorm:"type:varchar(100);"`
	V4    string `gorm:"type:varchar(100);"`
	V5    string `gorm:"type:varchar(100);"`
}

// TableName 方法.
func (CasbinRule) TableName() string {
	return "casbin_rule"
}
