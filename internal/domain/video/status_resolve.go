package video

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// ResolvePublishVideoStatus 根据私密/定时/内容审核配置解析投稿或改稿后的视频状态。
//
// 规则（与原 service/video_service/status_resolve.go 完全一致）：
//   - 私密 → Private
//   - 未来发布时间 + 开启审核 → Review（需人工过审，不可自动上架）
//   - 未来发布时间 + 关闭审核 → Scheduled（到点由调度器发布）
//   - 立即公开 + 关闭审核 → Published
//   - 立即公开 + 开启审核 → Review
//
// 原实现直接读 global.Config()；新架构把「是否开启内容审核」上提为显式入参，
// 由应用层从注入的 *config.Provider 读取后传入，领域层因此零框架依赖。
func ResolvePublishVideoStatus(
	now time.Time,
	publishTime *time.Time,
	isPrivate bool,
	contentReviewEnabled bool,
) enum.VideoStatus {
	if isPrivate {
		return enum.VideoStatusPrivate
	}

	if publishTime != nil && publishTime.After(now) {
		if contentReviewEnabled {
			return enum.VideoStatusReview
		}

		return enum.VideoStatusScheduled
	}

	if !contentReviewEnabled {
		return enum.VideoStatusPublished
	}

	return enum.VideoStatusReview
}

// ShouldScheduleVideoPublish 是否应写入定时发布任务。
// 私密不调度；未来公开时间写入调度（开启审核时也保留分数，便于过审后按点发布）。
func ShouldScheduleVideoPublish(now, publishTime time.Time, isPrivate bool) bool {
	if isPrivate {
		return false
	}

	return publishTime.After(now)
}

// ScheduledPublishOutcome 定时任务到期后的处理结论。
type ScheduledPublishOutcome int

// 定时任务到期结论枚举。
const (
	// ScheduledPublishDrop 丢弃调度（已发布 / 私密 / 删除等终态）。
	ScheduledPublishDrop ScheduledPublishOutcome = iota
	// ScheduledPublishFromScheduled 从 Scheduled 状态发布。
	ScheduledPublishFromScheduled
	// ScheduledPublishFromReview 从 Review 状态发布（关闭审核时的历史数据）。
	ScheduledPublishFromReview
	// ScheduledPublishKeepReview 保留待审状态，仅丢弃调度。
	ScheduledPublishKeepReview
)

// ResolveScheduledPublishOutcome 判定到期定时任务应如何处理。
//
// 与原 processDueScheduledVideo 的 switch 分支逐项一致。
func ResolveScheduledPublishOutcome(status enum.VideoStatus, contentReviewEnabled bool) ScheduledPublishOutcome {
	switch status {
	case enum.VideoStatusScheduled:
		return ScheduledPublishFromScheduled
	case enum.VideoStatusReview:
		// 开启内容审核时 Review 不可自动上架，到期仅丢弃调度，保留待审状态。
		if contentReviewEnabled {
			return ScheduledPublishKeepReview
		}

		// 关闭审核时的历史数据：曾用 Review 表示定时待发。
		return ScheduledPublishFromReview
	default:
		// 已发布 / 私密 / 删除等：清理陈旧调度。
		return ScheduledPublishDrop
	}
}

// ValidateReviewTargetStatus 校验管理端审核目标状态是否合法。
func ValidateReviewTargetStatus(status enum.VideoStatus) error {
	switch status {
	case enum.VideoStatusPublished, enum.VideoStatusPrivate, enum.VideoStatusDeleted:
		return nil
	default:
		return ErrVideoStatusInvalid
	}
}
