package video

import (
	"context"
)

// StatsRepository 按业务能力声明的持久化契约。
type StatsRepository interface {
	// PublishedStats 读取已发布视频的统计行；不存在时返回 ErrRecordNotFound。
	PublishedStats(ctx context.Context, videoID uint) (*Stats, error)
	// ApplyStatsDelta 在 MySQL 上以「不小于 0」表达式应用统计增量（仅已发布视频）。
	ApplyStatsDelta(ctx context.Context, videoID uint, delta StatsDelta) error
}
