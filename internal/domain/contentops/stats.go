package contentops

import "time"

// ratePercentBase 环比增长率的百分比基数。
const ratePercentBase = 100

// StatsPolicy 计算运营统计的环比增长率。没有外部依赖，零值即可使用。
type StatsPolicy struct{}

type StatsPeriod struct {
	Start, End time.Time
}

type StatsWindows struct {
	Today   StatsPeriod
	Daily   []StatsPeriod
	Monthly []StatsPeriod
}

// Windows 定义最近七天、最近十二个月及今日的左闭右开区间。
func (StatsPolicy) Windows(now time.Time) StatsWindows {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	windows := StatsWindows{Today: StatsPeriod{Start: today, End: today.AddDate(0, 0, 1)}}
	for i := 6; i >= 0; i-- {
		start := today.AddDate(0, 0, -i)
		windows.Daily = append(windows.Daily, StatsPeriod{Start: start, End: start.AddDate(0, 0, 1)})
	}
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	for i := 11; i >= 0; i-- {
		start := month.AddDate(0, -i, 0)
		windows.Monthly = append(windows.Monthly, StatsPeriod{Start: start, End: start.AddDate(0, 1, 0)})
	}
	return windows
}

// CalcRates 计算相邻两期的环比增长率序列（百分比）。
//
// 规则：
//   - 首个数据点无上一期，记 0；
//   - 上一期非正时：本期为 0 记 0，否则记 100（视为从无到有的满增长）；
//   - 其余按 (本期-上期)*100/上期 计算。
func (StatsPolicy) CalcRates(values []int64) []float64 {
	rates := make([]float64, len(values))
	for i := range values {
		if i == 0 {
			rates[i] = 0

			continue
		}

		prev := values[i-1]

		curr := values[i]
		if prev <= 0 {
			if curr == 0 {
				rates[i] = 0
			} else {
				rates[i] = ratePercentBase
			}

			continue
		}

		rates[i] = float64(curr-prev) * ratePercentBase / float64(prev)
	}

	return rates
}
