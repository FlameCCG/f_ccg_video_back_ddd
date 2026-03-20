package entity

import (
	identityerrors "f_ccg_video_back_ddd/internal/domain/identity/domainerr"
	"f_ccg_video_back_ddd/pkg/coinutil"
)

// CoinAccount 仅承载硬币账户身份与受控余额，不替代用户资料读模型。
type CoinAccount struct {
	userID  uint
	balance float64
}

// RestoreCoinAccount 重建持久化账户，保留历史负余额及原始精度。
func RestoreCoinAccount(userID uint, balance float64) *CoinAccount {
	return &CoinAccount{userID: userID, balance: balance}
}
func (a *CoinAccount) UserID() uint     { return a.userID }
func (a *CoinAccount) Balance() float64 { return a.balance }

// ApplyChange 执行普通余额变动。消费不能新增或扩大欠款；
// 已有欠款允许通过正向收入逐步偿还。失败时保持原余额。
func (u *CoinAccount) ApplyChange(delta float64) error {
	if u == nil {
		return identityerrors.ErrUserNotFound
	}
	delta = coinutil.Round(delta)
	if delta == 0 {
		return nil
	}
	after := coinutil.Round(u.balance + delta)
	if delta < 0 && after < 0 {
		return identityerrors.ErrCoinInsufficient
	}
	u.balance = after
	return nil
}

// RevokeFavoriteReward 是奖励追回的明确例外：奖励已消费时允许形成欠款。
// 流水去重及余额锁定由应用层在同一事务内完成。
func (u *CoinAccount) RevokeFavoriteReward() error {
	if u == nil {
		return identityerrors.ErrUserNotFound
	}
	u.balance = coinutil.Round(u.balance - CoinFavoriteReward)
	return nil
}
