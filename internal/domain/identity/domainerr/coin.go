package domainerr

import "errors"

// ErrCoinInsufficient 硬币不足.
var ErrCoinInsufficient = errors.New("coin insufficient")

// IsCoinInsufficient 判断错误是否为「硬币余额不足」。
//
// 供其它上下文（如 video 三连流程）在不 import 本包哨兵错误的前提下做判定：
// 它们只在自己的 application 层声明一个 IsInsufficient(error) bool 的端口方法。
func IsCoinInsufficient(err error) bool {
	return errors.Is(err, ErrCoinInsufficient)
}
