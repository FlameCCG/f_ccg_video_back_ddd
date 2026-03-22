package identity

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// BanAction 封禁/解封指令（值对象）。字段保持公开。
type BanAction struct {
	UserID     uint
	Status     enum.UserStatus
	Days       int
	Reason     string
	OperatorID uint
}

// BanRecord 封禁记录实体。HTTP 响应使用独立 DTO，不序列化本实体。
type BanRecord struct {
	id         uint
	createdAt  time.Time
	updatedAt  time.Time
	userID     uint
	status     enum.UserStatus
	reason     string
	days       int
	startAt    time.Time
	endAt      *time.Time
	operatorID uint
	isUnbanned bool
}

// BanRecordSnapshot 是封禁记录的持久化快照。
type BanRecordSnapshot struct {
	ID         uint
	CreatedAt  time.Time
	UpdatedAt  time.Time
	UserID     uint
	Status     enum.UserStatus
	Reason     string
	Days       int
	StartAt    time.Time
	EndAt      *time.Time
	OperatorID uint
	IsUnbanned bool
}

// 封禁流程校验错误。
var (
	// ErrBanUserIDRequired userId 缺失.
	ErrBanUserIDRequired = errors.New("userId 不能为空")
	// ErrBanDaysRequired 限时封禁必须给出正的天数.
	ErrBanDaysRequired = errors.New("封禁天数必须大于0")
	// ErrBanStatusInvalid 目标状态非法.
	ErrBanStatusInvalid = errors.New("状态非法")
)

// Normalize 规范化封禁指令并推导目标状态。
//
// 规则：
//   - Reason 去首尾空白；
//   - Status 为 0 时按 Days 推导：Days>0 → 封禁中，否则 → 正常；
//   - 解封与永久封禁强制 Days=0；限时封禁要求 Days>0；
//   - 其它状态视为非法。
func (action BanAction) Normalize() (BanAction, error) {
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

// NewBanRecord 校验并规范化指令，再构造封禁记录（限时封禁计算解封时间）。
func NewBanRecord(action BanAction, now time.Time) (*BanRecord, error) {
	action, err := action.Normalize()
	if err != nil {
		return nil, err
	}
	var endAt *time.Time

	if action.Status == enum.UserStatusBanned {
		t := now.AddDate(0, 0, action.Days)
		endAt = &t
	}

	return &BanRecord{
		userID:     action.UserID,
		status:     action.Status,
		reason:     action.Reason,
		days:       action.Days,
		startAt:    now,
		endAt:      endAt,
		operatorID: action.OperatorID,
		isUnbanned: action.Status == enum.UserStatusActive,
	}, nil
}

// RestoreBanRecord 从快照恢复封禁记录，不重新计算解封时间。
func RestoreBanRecord(snapshot BanRecordSnapshot) *BanRecord {
	return &BanRecord{
		id:         snapshot.ID,
		createdAt:  snapshot.CreatedAt,
		updatedAt:  snapshot.UpdatedAt,
		userID:     snapshot.UserID,
		status:     snapshot.Status,
		reason:     snapshot.Reason,
		days:       snapshot.Days,
		startAt:    snapshot.StartAt,
		endAt:      (stateCopy{}).time(snapshot.EndAt),
		operatorID: snapshot.OperatorID,
		isUnbanned: snapshot.IsUnbanned,
	}
}

// Snapshot 返回脱离实体的副本。
func (r *BanRecord) Snapshot() BanRecordSnapshot {
	if r == nil {
		return BanRecordSnapshot{}
	}

	return BanRecordSnapshot{
		ID:         r.id,
		CreatedAt:  r.createdAt,
		UpdatedAt:  r.updatedAt,
		UserID:     r.userID,
		Status:     r.status,
		Reason:     r.reason,
		Days:       r.days,
		StartAt:    r.startAt,
		EndAt:      (stateCopy{}).time(r.endAt),
		OperatorID: r.operatorID,
		IsUnbanned: r.isUnbanned,
	}
}

func (r *BanRecord) ID() uint                { return r.orZero().id }
func (r *BanRecord) CreatedAt() time.Time    { return r.orZero().createdAt }
func (r *BanRecord) UpdatedAt() time.Time    { return r.orZero().updatedAt }
func (r *BanRecord) UserID() uint            { return r.orZero().userID }
func (r *BanRecord) Status() enum.UserStatus { return r.orZero().status }
func (r *BanRecord) Reason() string          { return r.orZero().reason }
func (r *BanRecord) Days() int               { return r.orZero().days }
func (r *BanRecord) StartAt() time.Time      { return r.orZero().startAt }
func (r *BanRecord) OperatorID() uint        { return r.orZero().operatorID }
func (r *BanRecord) IsUnbanned() bool        { return r.orZero().isUnbanned }

// EndAt 返回解封时间副本。
func (r *BanRecord) EndAt() *time.Time {
	if r == nil {
		return nil
	}

	return (stateCopy{}).time(r.endAt)
}

func (r *BanRecord) orZero() BanRecord {
	if r == nil {
		return BanRecord{}
	}

	return *r
}

// Release 标记原封禁记录已解封，保留其身份、原因和封禁历史。
func (r *BanRecord) Release() {
	if r != nil {
		r.isUnbanned = true
	}
}

// ApplyBan 同时建立封禁事件并改变用户状态，拒绝时两者均不变。
func (u *User) ApplyBan(action BanAction, now time.Time) (*BanRecord, error) {
	record, err := NewBanRecord(action, now)
	if err != nil {
		return nil, err
	}
	if u == nil || u.id != record.userID {
		return nil, ErrUserNotFound
	}
	u.status = record.status
	return record, nil
}

// ReleaseExpiredBan 只解除当前用户已到期的限时封禁，连同原记录一起改变。
func (u *User) ReleaseExpiredBan(record *BanRecord, now time.Time) bool {
	if !u.HasTimedBan() || !record.TimedBanOpen() || record.userID != u.id || record.isUnbanned || !record.DueForRelease(now) {
		return false
	}
	u.status = enum.UserStatusActive
	record.Release()
	return true
}

// EnsureCanPublish 返回当前封禁状态对应的内容发布资格。
func (u *User) EnsureCanPublish() error {
	if u == nil {
		return ErrUserNotFound
	}
	if u.IsPermanentlyBanned() {
		return ErrAccountPermanentlyBanned
	}
	if u.IsBanned() {
		return ErrAccountBanned
	}
	return nil
}

// TimedBanOpen 判断这是一条仍带解封时间的限时封禁记录。
func (r *BanRecord) TimedBanOpen() bool {
	return r != nil && r.status == enum.UserStatusBanned && r.endAt != nil
}

// EndPassed 判断解封时间是否严格早于 now。
func (r *BanRecord) EndPassed(now time.Time) bool {
	return r != nil && r.endAt != nil && r.endAt.Before(now)
}

// DueForRelease 判断解封时间已到（等于 now 也算到期）。无解封时间则不到期。
func (r *BanRecord) DueForRelease(now time.Time) bool {
	return r != nil && r.endAt != nil && !r.endAt.After(now)
}

// Notice 组装封禁通知正文。
func (r *BanRecord) Notice() string {
	if r == nil {
		return ""
	}

	content := ""

	switch r.status {
	case enum.UserStatusActive:
		content = "您的账号已解除封禁。"
	case enum.UserStatusBanned:
		content = fmt.Sprintf("您的账号已被封禁 %d 天。", r.days)
		if r.endAt != nil {
			content += fmt.Sprintf("解封时间：%s。", r.endAt.Format("2006-01-02 15:04:05"))
		}
	case enum.UserStatusPermanentlyBanned:
		content = "您的账号已被永久封禁。"
	}

	if r.reason != "" {
		content += "原因：" + r.reason
	}

	return content
}
