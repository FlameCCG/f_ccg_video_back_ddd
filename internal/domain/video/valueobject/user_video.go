package valueobject

import "f_ccg_video_back_ddd/internal/domain/shared/enum"

// MapUserVideoAuditStatus 将视频状态映射为用户主页的审核态编码和文案，仅本人查看时展示。
func MapUserVideoAuditStatus(status enum.VideoStatus) (int, string) {
	switch status {
	case enum.VideoStatusPublished:
		return UserVideoAuditPublished, "已发布"
	case enum.VideoStatusPrivate:
		return UserVideoAuditPrivate, "私密"
	case enum.VideoStatusReview:
		return UserVideoAuditReviewing, "审核中"
	case enum.VideoStatusScheduled:
		return UserVideoAuditScheduled, "定时发布"
	default:
		return 0, ""
	}
}

// MapUserVideoStatusText 把视频状态映射为用户主页展示文案。
//
// 文案。
func MapUserVideoStatusText(status enum.VideoStatus) string {
	switch status {
	case enum.VideoStatusPublished:
		return "已发布"
	case enum.VideoStatusPrivate:
		return "私密"
	case enum.VideoStatusDeleted:
		return "已删除"
	case enum.VideoStatusReview:
		return "审核中"
	case enum.VideoStatusScheduled:
		return "定时发布"
	default:
		return ""
	}
}

// NormalizeUserVideoPaging 规范用户主页视频列表的分页参数。
//
// 页码下限 1，页大小超出 (0, 50] 时回落 20。
func NormalizeUserVideoPaging(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}

	const maxUserVideoPageSize = 50
	if pageSize <= 0 || pageSize > maxUserVideoPageSize {
		pageSize = 20
	}

	return page, pageSize
}

// NormalizeRecentActionLimit 规范「最近点赞/投币」的取数上限。
//
// 超出 (0, 50] 时回落 10。
func NormalizeRecentActionLimit(limit int) int {
	const maxRecentActionLimit = 50
	if limit <= 0 || limit > maxRecentActionLimit {
		return 10
	}

	return limit
}
