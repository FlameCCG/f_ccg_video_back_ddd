package chat

import "time"

// NeedsDailySendLimitCheck 判断是否需要进入单日发送频率判定。
//
// 与原 checkDailySendLimit 的前置 guard 一致：缺少任一方或自己给自己发时直接放行。
func NeedsDailySendLimitCheck(senderID, receiverID uint) bool {
	return senderID != 0 && receiverID != 0 && senderID != receiverID
}

// DayRange 返回 now 所在自然日的 [起始, 次日起始) 区间（沿用 now 的时区）。
func DayRange(now time.Time) (time.Time, time.Time) {
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	return dayStart, dayStart.AddDate(0, 0, 1)
}

// EvaluateDailySendLimit 判定单日发送限制。
//
// 规则（对端未关注自己时生效）：今日已发过一条且对端今日未回复 → 拒绝；
// 其余情形放行。
func EvaluateDailySendLimit(sentToday, repliedToday bool) error {
	if !sentToday {
		return nil
	}

	if repliedToday {
		return nil
	}

	return ErrDailySendLimit
}
