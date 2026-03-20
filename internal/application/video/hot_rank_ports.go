package video

import (
	"context"
)

// HotRankCache 视频热度 ZSet 端口。
type HotRankCache interface {
	// Available 报告 Redis 是否可用。
	Available() bool
	// IncrBy 累加热度分。
	IncrBy(ctx context.Context, videoID uint, score float64)
	// Card 读取 ZSet 基数。
	Card(ctx context.Context) (int64, error)
	// RevRange 倒序取排名区间的视频 ID。
	RevRange(ctx context.Context, start, stop int64) ([]uint, error)
	// Remove 从 ZSet 移除视频。
	Remove(ctx context.Context, videoIDs []uint)
}
