package platform

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// AutoUnbanReason 自动解封记录写入的原因文案（与原实现逐字一致）。
const AutoUnbanReason = "自动解封"

// ExpiredBan 一条到期的限时封禁：最新封禁记录仍为封禁中且结束时间已过。
type ExpiredBan struct {
	// UserID 被封禁用户 ID。
	UserID uint
	// EndAt 封禁结束时间；到期判定已在查询侧完成，此处仅作留痕。
	EndAt *time.Time
}

// UnbanRecord 自动解封时补写的封禁记录（状态回到正常）。
type UnbanRecord struct {
	// UserID 被解封用户 ID。
	UserID uint
	// Status 解封后的用户状态。
	Status enum.UserStatus
	// Reason 解封原因。
	Reason string
	// Days 封禁天数，自动解封固定为 0。
	Days int
	// StartAt 记录生成时间。
	StartAt time.Time
	// OperatorID 操作人 ID，系统自动解封固定为 0。
	OperatorID uint
}

// AutoUnbanPlan 一次自动解封的执行计划：待解封用户与需补写的记录。
type AutoUnbanPlan struct {
	// UserIDs 待解封用户 ID 列表，顺序与查询结果一致。
	UserIDs []uint
	// Records 与 UserIDs 一一对应的解封记录。
	Records []UnbanRecord
}

// Empty 报告计划是否无事可做。
func (p AutoUnbanPlan) Empty() bool {
	return len(p.UserIDs) == 0
}

// PlanAutoUnban 由到期封禁列表推导自动解封计划。
//
// 规则与原 cron_service.AutoUnbanUsers 一致：跳过非法用户 ID（0），
// 为每个待解封用户生成一条「正常状态 / 原因自动解封 / 天数 0 / 操作人系统」的记录，
// 记录时间统一取本轮的 now，保证同批记录时间一致。
func PlanAutoUnban(expired []ExpiredBan, now time.Time) AutoUnbanPlan {
	if len(expired) == 0 {
		return AutoUnbanPlan{}
	}

	plan := AutoUnbanPlan{
		UserIDs: make([]uint, 0, len(expired)),
		Records: make([]UnbanRecord, 0, len(expired)),
	}

	for _, ban := range expired {
		if ban.UserID == 0 {
			continue
		}

		plan.UserIDs = append(plan.UserIDs, ban.UserID)
		plan.Records = append(plan.Records, UnbanRecord{
			UserID:     ban.UserID,
			Status:     enum.UserStatusActive,
			Reason:     AutoUnbanReason,
			Days:       0,
			StartAt:    now,
			OperatorID: 0,
		})
	}

	if len(plan.UserIDs) == 0 {
		return AutoUnbanPlan{}
	}

	return plan
}
