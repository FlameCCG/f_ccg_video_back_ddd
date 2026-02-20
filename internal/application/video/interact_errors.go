package video

import (
	"errors"

	domain "f_ccg_video_back_ddd/internal/domain/video"
)

// IsCoinInsufficient 判断硬币余额不足，并将 identity 的错误映射为 video 上下文的哨兵。
func (s *InteractionService) IsCoinInsufficient(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, domain.ErrCoinInsufficient) {
		return true
	}

	return s.ledger != nil && s.ledger.IsInsufficient(err)
}
