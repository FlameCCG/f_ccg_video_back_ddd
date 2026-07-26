package video

import (
	"context"
	"errors"
	"strings"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
)

// videoStatusPublished 已发布状态别名，减少各处 enum 引用噪音。
const videoStatusPublished = enum.VideoStatusPublished

// orTODO 兜底 nil context。
func orTODO(ctx context.Context) context.Context {
	if ctx == nil {
		return context.TODO()
	}

	return ctx
}

// trimSpace 裁剪两端空白。
func trimSpace(s string) string {
	return strings.TrimSpace(s)
}

// isRecordNotFound 判断错误是否为「记录不存在」。
func isRecordNotFound(err error) bool {
	return errors.Is(err, domain.ErrRecordNotFound)
}

// uniqueVideoIDs 去零去重，保持首次出现顺序。
func uniqueVideoIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return nil
	}

	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))

	for _, id := range ids {
		if id == 0 {
			continue
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		out = append(out, id)
	}

	return out
}

// applyCachedStatsToFeedItem 用 Redis 权威计数覆盖列表项的播放量与弹幕数。
func applyCachedStatsToFeedItem(item FeedItem, stats *domain.Stats) FeedItem {
	if stats != nil {
		item.Views = stats.Views
		item.DanmuCount = stats.DanmuCount
	}

	return item
}

// mapUserVideoStatusText 视频状态的用户可见文案（与原 mapUserVideoStatusText 一致）。
func mapUserVideoStatusText(status enum.VideoStatus) string {
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

// mapUserVideoAuditStatus 视频状态到作者视角审核态的映射（与原实现一致）。
func mapUserVideoAuditStatus(status enum.VideoStatus) (int, string) {
	switch status {
	case enum.VideoStatusPublished:
		return domain.UserVideoAuditPublished, "已发布"
	case enum.VideoStatusPrivate:
		return domain.UserVideoAuditPrivate, "私密"
	case enum.VideoStatusReview:
		return domain.UserVideoAuditReviewing, "审核中"
	case enum.VideoStatusScheduled:
		return domain.UserVideoAuditScheduled, "定时发布"
	default:
		return 0, ""
	}
}
