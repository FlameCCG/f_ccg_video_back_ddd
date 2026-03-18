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

// applyCachedStatsToFeedItem 用 Redis 权威计数覆盖列表项的播放量与弹幕数。
func applyCachedStatsToFeedItem(item FeedItem, stats *domain.Stats) FeedItem {
	if stats != nil {
		item.Views = stats.Views
		item.DanmuCount = stats.DanmuCount
	}

	return item
}
