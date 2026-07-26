// Package coinutil provides package functionality.
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
