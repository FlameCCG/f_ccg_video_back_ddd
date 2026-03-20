package identity

import (
	"time"
)

// CoinRecordItem 硬币流水列表项（读模型）.
type CoinRecordItem struct {
	ID        uint      `json:"id"`
	Delta     float64   `json:"delta"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"createdAt"`
}

// ExpRecordItem 经验流水列表项（读模型）.
type ExpRecordItem struct {
	ID        uint      `json:"id"`
	Delta     int64     `json:"delta"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"createdAt"`
}

// LoginRecordItem 登录记录列表项（读模型）.
type LoginRecordItem struct {
	ID        uint      `json:"id"`
	IP        string    `json:"ip"`
	Addr      string    `json:"addr"`
	UserAgent string    `json:"userAgent"`
	LoginType string    `json:"loginType"`
	CreatedAt time.Time `json:"createdAt"`
}
