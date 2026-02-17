package po

// UserCoinRecord 用户硬币变动/操作记录.
type UserCoinRecord struct {
	Model
	UserID   uint    `gorm:"not null;index;uniqueIndex:idx_user_coin_dedup,priority:1;comment:用户ID" json:"userId"`
	Delta    float64 `gorm:"type:decimal(10,1);not null;comment:硬币数量变化(支持1位小数,可为负)" json:"delta"`
	Reason   string  `gorm:"size:255;not null;comment:原因" json:"reason"`
	DedupKey *string `gorm:"size:64;uniqueIndex:idx_user_coin_dedup,priority:2;comment:幂等键(可空)" json:"-"`
}

// TableName 方法.
func (UserCoinRecord) TableName() string {
	return "user_coin_records"
}
