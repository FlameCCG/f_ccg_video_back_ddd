package entity

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
