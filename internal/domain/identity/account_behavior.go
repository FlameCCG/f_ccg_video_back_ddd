package identity

// ApplyExperience 维护经验和等级，并根据当日累计裁剪正向奖励。
//
// 返回实际记账增量；负增量按请求值记录，经验结果最低为零。拒绝时不修改经验与等级。
func (u *User) ApplyExperience(delta, gainedToday int64) (applied int64, allowed bool) {
	if u == nil || delta == 0 {
		return 0, false
	}

	policy := ExperiencePolicy{}
	applied, allowed = policy.ClampDaily(delta, gainedToday)
	if !allowed {
		return 0, false
	}

	u.exp = max(u.exp+applied, 0)
	u.level = policy.Level(u.exp)

	return applied, true
}
