package po

// UserExpRecord 用户经验变动记录.
type UserExpRecord struct {
	Model
	UserID   uint    `gorm:"not null;index;uniqueIndex:idx_user_exp_dedup,priority:1;comment:用户ID" json:"userId"`
	Delta    int64   `gorm:"not null;comment:经验变化" json:"delta"`
	Reason   string  `gorm:"size:255;not null;comment:原因" json:"reason"`
	DedupKey *string `gorm:"size:64;uniqueIndex:idx_user_exp_dedup,priority:2;comment:幂等键(可空)" json:"-"`
}

// TableName 显式绑定表名 user_exp_records，保持迁移与仓储 SQL 使用同一张表。
func (UserExpRecord) TableName() string {
	return "user_exp_records"
}
