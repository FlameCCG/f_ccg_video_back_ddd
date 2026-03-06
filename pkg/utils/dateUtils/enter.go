// Package date 计算当前进程时区下的当天结束时刻。
//
// 函数使用当前进程时钟与时区，业务日窗口若要求固定时区应由所属用例显式计算。
// 不要把进程本地零点直接等同于上传配额或其他业务时区的自然日。
package date

import "time"

// TimeLeftInDay 返回当天的结束时间.
func TimeLeftInDay() time.Time {
	now := time.Now()
	location := now.Location()
	endTime := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, location)

	return endTime
}
