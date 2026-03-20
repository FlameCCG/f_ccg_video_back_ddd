package entity

import "errors"

// 房间弹幕分段查询相关错误。
var errEndRequired = errors.New("end 不能为空")
var errEndMustBeGreater = errors.New("end 必须大于 start")
var errDateFormat = errors.New("date 格式应为 YYYY-MM-DD")
var errMonthFormat = errors.New("month 格式应为 YYYY-MM")
var errInstantRange = errors.New("start/end 应为 RFC3339 时间，且间隔不超过 32 天")
