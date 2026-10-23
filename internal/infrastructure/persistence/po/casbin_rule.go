package po

// CasbinRule 保存 Casbin 策略行，Ptype 决定规则类型，V0～V5 的位置含义由 conf/rbac_model.conf 定义。
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

// TableName 显式绑定表名 casbin_rule，保持迁移与仓储 SQL 使用同一张表。
func (CasbinRule) TableName() string {
	return "casbin_rule"
}
