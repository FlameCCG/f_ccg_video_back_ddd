package dynamic

import (
	"context"
	"time"
)

// VideoPinWriter 视频置顶写侧端口（由 video 上下文实现）。
//
// 动态与视频的置顶互斥规则要求本上下文能取消作者已置顶的视频，
// 但视频聚合的写侧归 video 上下文所有，故以端口协作。
type VideoPinWriter interface {
	// ClearPinnedByAuthor 取消该作者当前已置顶的全部视频。
	ClearPinnedByAuthor(ctx context.Context, authorUsername string) error
	// SetPinned 把作者名下已发布的视频置顶，并写入时间。目标不再符合条件时返回 false。
	SetPinned(ctx context.Context, videoID uint, authorUsername string, pinnedAt *time.Time) (bool, error)
	// ClearPinned 取消作者名下已发布的视频置顶。重复取消返回 true，目标失效返回 false。
	ClearPinned(ctx context.Context, videoID uint, authorUsername string) (bool, error)
}
