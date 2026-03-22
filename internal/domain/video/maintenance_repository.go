package video

import (
	"context"
)

type VideoStatsProjection interface {
	// PersistStats 将统计快照回写到已发布视频行。
	PersistStats(ctx context.Context, videoID uint, stats Stats) error
}
