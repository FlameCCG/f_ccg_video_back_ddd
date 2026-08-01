package identity

import "errors"

// 硬币奖励规则（原 service/user_service/coin_service.go 中的常量）。
const (
	// CoinLoginReward 每日登录硬币奖励.
	CoinLoginReward float64 = 1.0
	// CoinFavoriteReward 作品被收藏时作者获得的硬币奖励.
	CoinFavoriteReward float64 = 0.1
	// CoinRewardRateOfCoined 作品被投币时作者获得的奖励比例（投币数的 10%）.
	CoinRewardRateOfCoined float64 = 0.1
)

// ErrCoinInsufficient 硬币不足.
var ErrCoinInsufficient = errors.New("coin insufficient")

// IsCoinInsufficient 判断错误是否为「硬币余额不足」。
//
// 供其它上下文（如 video 三连流程）在不 import 本包哨兵错误的前提下做判定：
// 它们只在自己的 application 层声明一个 IsInsufficient(error) bool 的端口方法。
func IsCoinInsufficient(err error) bool {
	return errors.Is(err, ErrCoinInsufficient)
}
