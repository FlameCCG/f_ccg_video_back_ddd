package video

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// Revision 是作者改稿的业务意图；nil 字段保留原值，不包含数据库列名。
type Revision struct {
	Title       *string
	Description *string
	Cover       *string
	PartitionID *uint
	IsOriginal  *bool
	IsPrivate   *bool
	PublishTime *time.Time
}

// Revise 保护作者身份和可编辑状态，再应用改稿。删除后的恢复仅属于管理端用例。
// now 和审核开关由调用方提供，领域不读取时钟、配置或调度存储。
func (v *Video) Revise(author string, change Revision, now time.Time, reviewEnabled bool) error {
	if v == nil || !v.OwnedBy(author) || v.IsDeleted() {
		return ErrRecordNotFound
	}

	switch v.Status {
	case enum.VideoStatusPublished, enum.VideoStatusPrivate, enum.VideoStatusReview, enum.VideoStatusScheduled:
	default:
		return ErrVideoStatusInvalid
	}

	if change.Title != nil {
		v.Title = *change.Title
	}
	if change.Description != nil {
		v.Description = *change.Description
	}
	if change.Cover != nil {
		v.Cover = *change.Cover
	}
	if change.PartitionID != nil {
		v.PartitionID = *change.PartitionID
	}
	if change.IsOriginal != nil {
		v.IsOriginal = *change.IsOriginal
	}

	if change.IsPrivate != nil || change.PublishTime != nil {
		isPrivate := v.Status == enum.VideoStatusPrivate
		if change.IsPrivate != nil {
			isPrivate = *change.IsPrivate
		}
		v.Status = ResolvePublishVideoStatus(now, change.PublishTime, isPrivate, reviewEnabled)
	}

	return nil
}
