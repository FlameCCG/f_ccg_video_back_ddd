package entity

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	videoerrors "f_ccg_video_back_ddd/internal/domain/video/domainerr"
)

// VideoDraft 只包含作者可以提供的稿件信息；状态、审核时间及统计由模型决定。
type VideoDraft struct {
	Title, Description, Cover, AuthorUsername string
	PartitionID                               uint
	IsOriginal                                bool
	StorageType                               enum.VideoStorageType
}

// NewVideo 同时建立稿件状态与定时意图，避免调用方独立拼装两者。
func NewVideo(draft VideoDraft, publishTime time.Time, isPrivate, reviewEnabled bool, now time.Time) (*Video, *time.Time) {
	v := &Video{
		Title: draft.Title, Description: draft.Description, Cover: draft.Cover,
		AuthorUsername: draft.AuthorUsername, PartitionID: draft.PartitionID,
		IsOriginal: draft.IsOriginal, StorageType: draft.StorageType,
		Status: ResolvePublishVideoStatus(now, &publishTime, isPrivate, reviewEnabled),
	}
	if ShouldScheduleVideoPublish(now, publishTime, isPrivate) {
		v.IsScheduled = true
		return v, &publishTime
	}
	return v, nil
}

// Review 只对待审稿件执行审核。通过时统一决定立即发布或定时发布并记录通过时间。
// 返回 false 表示当前状态无需处理，保持批量审核的幂等语义。
func (v *Video) Review(target enum.VideoStatus, publishAt *time.Time, now time.Time) (bool, error) {
	if err := ValidateReviewTargetStatus(target); err != nil {
		return false, err
	}
	if v == nil || v.Status != enum.VideoStatusReview {
		return false, nil
	}
	if target == enum.VideoStatusPublished {
		if publishAt != nil && publishAt.After(now) {
			target = enum.VideoStatusScheduled
		}
		v.ApprovedAt = &now
	}
	v.Status = target
	return true, nil
}

// DuePublication 是处理一次定时意图后的结论；存储操作由用例在事务中执行。
type DuePublication struct {
	Handled    bool
	Published  bool
	RemovePlan bool
}

// PublishWhenDue 在最新状态和计划上决定是否到期发布；待审与未来计划均保持不变。
func (v *Video) PublishWhenDue(publishAt *time.Time, now time.Time, reviewEnabled bool) DuePublication {
	if v == nil {
		return DuePublication{Handled: true, RemovePlan: true}
	}
	if v.IsPublished() || publishAt == nil {
		return DuePublication{Handled: true}
	}
	if publishAt.After(now) {
		return DuePublication{}
	}
	switch ResolveScheduledPublishOutcome(v.Status, reviewEnabled) {
	case ScheduledPublishFromScheduled, ScheduledPublishFromReview:
		v.Status = enum.VideoStatusPublished
		return DuePublication{Handled: true, Published: true}
	case ScheduledPublishKeepReview:
		return DuePublication{}
	default:
		return DuePublication{Handled: true, RemovePlan: true}
	}
}

// DeleteBy 执行作者软删除；已删除稿件重复执行不产生新的变更。
func (v *Video) DeleteBy(author string) (bool, error) {
	if v == nil {
		return false, videoerrors.ErrRecordNotFound
	}
	if !v.OwnedBy(author) {
		return false, videoerrors.ErrVideoDeleteForbidden
	}
	return v.MarkDeleted(), nil
}

// MarkDeleted 执行软删除状态变更；管理端授权由应用入口提供。
func (v *Video) MarkDeleted() bool {
	if v == nil {
		return false
	}
	if v.IsDeleted() {
		return false
	}
	v.Status = enum.VideoStatusDeleted
	return true
}

// Restore 只恢复已删除稿件，其它状态不受批量操作影响。
//
// 恢复后立即公开，保留已有审核和发布历史。
func (v *Video) Restore() bool {
	if v == nil || !v.IsDeleted() {
		return false
	}
	v.Status = enum.VideoStatusPublished
	return true
}
