package video

import (
	"errors"

	domain "f_ccg_video_back_ddd/internal/domain/video"
)

// IsCoinInsufficient 判断错误是否为「硬币余额不足」。
//
// 硬币账本归 identity 上下文，其哨兵错误实例不能被 video 直接引用（依赖方向禁止），
// 因此判定经 CoinLedger 端口的 IsInsufficient 暴露；同时兼容 video 领域自己的
// ErrCoinInsufficient 哨兵，便于测试注入 fake 账本。
//
// 原实现里 video_service.ErrCoinInsufficient 是 user_service.ErrCoinInsufficient 的别名，
// handler 直接 errors.Is 即可；新架构下两个上下文各有独立哨兵，故需要本方法转译。
func (s *InteractionService) IsCoinInsufficient(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, domain.ErrCoinInsufficient) {
		return true
	}

	return s.ledger != nil && s.ledger.IsInsufficient(err)
}
