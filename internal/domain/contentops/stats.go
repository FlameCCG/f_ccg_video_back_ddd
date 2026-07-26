package contentops

// ratePercentBase 环比增长率的百分比基数。
const ratePercentBase = 100

// CalcRates 计算相邻两期的环比增长率序列（百分比）。
//
// 规则与原 service/site_service/admin_stats_service.go 的 calcRates 逐行一致：
//   - 首个数据点无上一期，记 0；
//   - 上一期非正时：本期为 0 记 0，否则记 100（视为从无到有的满增长）；
//   - 其余按 (本期-上期)*100/上期 计算。
func CalcRates(values []int64) []float64 {
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
