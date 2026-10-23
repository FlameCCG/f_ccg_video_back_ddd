// Package coinutil 统一硬币展示的舍入和格式。
//
// Round 保留一位小数，Format 输出固定一位小数的字符串。
// 这是浮点值的显示/计算辅助，不提供数据库事务或高精度货币记账保证。
package coinutil

import (
	"math"
	"strconv"
)

const precision = 10

// Round 将硬币数统一保留 1 位小数。
func Round(value float64) float64 {
	return math.Round(value*precision) / precision
}

// Format 将硬币数格式化为固定 1 位小数。
func Format(value float64) string {
	return strconv.FormatFloat(Round(value), 'f', 1, 64)
}
