// Package date provides package functionality.
package date

import "time"

// TimeLeftInDay 返回当天的结束时间.
func TimeLeftInDay() time.Time {
	now := time.Now()
	location := now.Location()
	endTime := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, location)

	return endTime
}
