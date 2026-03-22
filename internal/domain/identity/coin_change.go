package identity

import "f_ccg_video_back_ddd/pkg/coinutil"

// CoinChange 是按既有一位小数规则归一化后的硬币变动值。
// 余额动作与流水使用同一个值；历史余额仍由 RestoreCoinAccount 原样重建。
type CoinChange struct {
	amount float64
}

func NewCoinChange(amount float64) CoinChange {
	return CoinChange{amount: coinutil.Round(amount)}
}

func (c CoinChange) Amount() float64 { return c.amount }
func (c CoinChange) IsZero() bool    { return c.amount == 0 }
