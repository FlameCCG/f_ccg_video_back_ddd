package entity

import (
	identityerrors "f_ccg_video_back_ddd/internal/domain/identity/domainerr"
	"f_ccg_video_back_ddd/pkg/coinutil"
)

// ApplyCoinChange 执行普通余额变动。消费不能新增或扩大欠款；
// 已有欠款允许通过正向收入逐步偿还。失败时保持原余额。
func (u *User) ApplyCoinChange(delta float64) error {
	if u == nil {
		return identityerrors.ErrUserNotFound
	}
	delta = coinutil.Round(delta)
	if delta == 0 {
		return nil
	}
	after := coinutil.Round(u.CoinCount + delta)
	if delta < 0 && after < 0 {
		return identityerrors.ErrCoinInsufficient
	}
	u.CoinCount = after
	return nil
}

// RevokeFavoriteReward 是奖励追回的明确例外：奖励已消费时允许形成欠款。
// 流水去重及余额锁定由应用层在同一事务内完成。
func (u *User) RevokeFavoriteReward() error {
	if u == nil {
		return identityerrors.ErrUserNotFound
	}
	u.CoinCount = coinutil.Round(u.CoinCount - CoinFavoriteReward)
	return nil
}

// ApplyExperience 维护经验和等级，并根据当日累计裁剪正向奖励。
//
// 返回实际记账增量；负增量按请求值记录，经验结果最低为零。
func (u *User) ApplyExperience(delta, gainedToday int64) (applied int64, allowed bool) {
	if u == nil || delta == 0 {
		return 0, false
	}
	applied, allowed = ClampDailyExpGain(delta, gainedToday)
	if !allowed {
		return 0, false
	}
	u.Exp = max(u.Exp+applied, 0)
	u.Level = CalcLevel(u.Exp)
	return applied, true
}
