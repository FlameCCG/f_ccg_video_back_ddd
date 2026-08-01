package platform

import (
	"time"
)

// ExpiredBan 一条到期的限时封禁：最新封禁记录仍为封禁中且结束时间已过。
type ExpiredBan struct {
	// RecordID 原封禁记录 ID。
	RecordID uint
	// UserID 被封禁用户 ID。
	UserID uint
	// EndAt 封禁结束时间；到期判定已在查询侧完成，此处仅作留痕。
	EndAt *time.Time
}

// AutoUnbanPlan 一次自动解封的执行计划：待解封用户与对应原封禁记录。
type AutoUnbanPlan struct {
	// UserIDs 待解封用户 ID 列表，顺序与查询结果一致。
	UserIDs []uint
	// RecordIDs 与 UserIDs 一一对应的原封禁记录 ID。
	RecordIDs []uint
}

// Empty 报告计划是否无事可做。
func (p AutoUnbanPlan) Empty() bool {
	return len(p.UserIDs) == 0
}

// PlanAutoUnban 由到期封禁列表推导自动解封计划。
//
// 跳过非法用户或记录 ID；自动解封只标记原封禁记录，不再补写 Active 记录。
func PlanAutoUnban(expired []ExpiredBan) AutoUnbanPlan {
	if len(expired) == 0 {
		return AutoUnbanPlan{}
	}

	plan := AutoUnbanPlan{
		UserIDs:   make([]uint, 0, len(expired)),
		RecordIDs: make([]uint, 0, len(expired)),
	}

	for _, ban := range expired {
		if ban.UserID == 0 || ban.RecordID == 0 {
			continue
		}

		plan.UserIDs = append(plan.UserIDs, ban.UserID)
		plan.RecordIDs = append(plan.RecordIDs, ban.RecordID)
	}

	if len(plan.UserIDs) == 0 {
		return AutoUnbanPlan{}
	}

	return plan
}
