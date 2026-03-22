package runtime

import (
	"context"
	mediadomain "f_ccg_video_back_ddd/internal/domain/media"
	"time"
)

// ProgressStore 转码进度写端口（Redis 适配器实现）。
type ProgressStore interface {
	// Write 写入进度快照并广播变更；ttl 由调用方按终态与否给出。
	Write(ctx context.Context, snapshot *mediadomain.TranscodeProgress, ttl time.Duration) error
	// Clear 删除进度快照（视频被删除时调用）。
	Clear(ctx context.Context, videoID, partID uint)
}
