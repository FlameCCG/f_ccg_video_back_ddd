package identity

import (
	"errors"
	"strings"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// BanAction 封禁/解封指令（值对象）。
type BanAction struct {
	UserID     uint
	Status     enum.UserStatus
	Days       int
	Reason     string
	OperatorID uint
}

// BanRecord 封禁记录实体。
//
// json tag 与原 models/mysqlEntity.UserBanRecord 完全一致，保证响应 JSON 1:1。
type BanRecord struct {
	ID         uint            `json:"id"`
	CreatedAt  time.Time       `json:"-"`
	UpdatedAt  time.Time       `json:"-"`
	UserID     uint            `json:"userId"`
	Status     enum.UserStatus `json:"status"`
	Reason     string          `json:"reason"`
	Days       int             `json:"days"`
	StartAt    time.Time       `json:"startAt"`
	EndAt      *time.Time      `json:"endAt,omitempty"`
	OperatorID uint            `json:"operatorId"`
}

// 封禁流程校验错误（消息与原实现逐字一致，保证接口错误文案不变）。
var (
	// ErrBanUserIDRequired userId 缺失.
	ErrBanUserIDRequired = errors.New("userId 不能为空")
	// ErrBanDaysRequired 限时封禁必须给出正的天数.
	ErrBanDaysRequired = errors.New("封禁天数必须大于0")
	// ErrBanStatusInvalid 目标状态非法.
	ErrBanStatusInvalid = errors.New("状态非法")
)

// NormalizeBanAction 规范化封禁指令并推导目标状态。
//
// 规则与原 BanService.Apply 完全一致：
//   - Reason 去首尾空白；
//   - Status 为 0 时按 Days 推导：Days>0 → 封禁中，否则 → 正常；
//   - 解封与永久封禁强制 Days=0；限时封禁要求 Days>0；
//   - 其它状态视为非法。
func NormalizeBanAction(action BanAction) (BanAction, error) {
	if action.UserID == 0 {
		return action, ErrBanUserIDRequired
	}

	action.Reason = strings.TrimSpace(action.Reason)

	if action.Status == 0 {
		if action.Days > 0 {
			action.Status = enum.UserStatusBanned
		} else {
			action.Status = enum.UserStatusActive
		}
	}

	switch action.Status {
	case enum.UserStatusActive:
		action.Days = 0
	case enum.UserStatusBanned:
		if action.Days <= 0 {
			return action, ErrBanDaysRequired
		}
	case enum.UserStatusPermanentlyBanned:
		action.Days = 0
	default:
		return action, ErrBanStatusInvalid
	}

	return action, nil
}

// NewBanRecord 依据规范化后的指令构造封禁记录（限时封禁计算解封时间）。
func NewBanRecord(action BanAction, now time.Time) *BanRecord {
	var endAt *time.Time

	if action.Status == enum.UserStatusBanned {
		t := now.AddDate(0, 0, action.Days)
		endAt = &t
	}

	return &BanRecord{
		UserID:     action.UserID,
		Status:     action.Status,
		Reason:     action.Reason,
		Days:       action.Days,
		StartAt:    now,
		EndAt:      endAt,
		OperatorID: action.OperatorID,
	}
}
