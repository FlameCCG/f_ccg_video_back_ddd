package identity

// 等级与经验规则（原 service/user_service/exp_service.go 中的常量与算法）。
const (
	// LevelMax 常量.
	LevelMax = 6
	// DailyExpCap 每用户每日经验获取总上限（登录/观看/点赞/投币等所有正向加经验共用）.
	DailyExpCap = 60
	// ExpPerLogin 常量.
	ExpPerLogin = 10
	// ExpPerLike 常量.
	ExpPerLike = 10
	// ExpPerCoin 单枚硬币投币折算经验.
	ExpPerCoin = 15
	// ExpPerWatch 每日观看视频经验.
	ExpPerWatch = 10

	level2Need = 1000
	level3Need = 3000
	level4Need = 6000
	level5Need = 10000
	level6Need = 20000
)

// CalcLevel 根据经验计算等级（1-6）.
func CalcLevel(exp int64) int {
	level := 1
	levelSteps := [...]int64{0, level2Need, level3Need, level4Need, level5Need, level6Need}

	for i := 1; i < len(levelSteps); i++ {
		if exp >= levelSteps[i] {
			level = i + 1
		}
	}

	if level > LevelMax {
		level = LevelMax
	}

	return level
}

// ClampDailyExpGain 依据当日已获得的正向经验，裁剪本次可生效的经验增量。
//
// 语义与原实现一致：
//   - 负向调整不占用日额度，原样返回；
//   - 当日额度耗尽时返回 (0, false)，调用方应放弃本次变动；
//   - 超出剩余额度时截断到剩余额度。
func ClampDailyExpGain(delta, gainedToday int64) (applied int64, allowed bool) {
	if delta <= 0 {
		return delta, true
	}

	remaining := int64(DailyExpCap) - gainedToday
	if remaining <= 0 {
		return 0, false
	}

	if delta > remaining {
		return remaining, true
	}

	return delta, true
}
