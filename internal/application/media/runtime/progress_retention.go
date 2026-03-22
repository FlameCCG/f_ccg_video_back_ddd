package runtime

import (
	"time"

	mediadomain "f_ccg_video_back_ddd/internal/domain/media"
)

// 进度缓存保留时间是运行时契约，保持已有 Redis TTL。
const (
	ProgressRunningTTL = 2 * time.Hour
	ProgressFinalTTL   = 24 * time.Hour
)

func ProgressTTL(snapshot mediadomain.TranscodeProgress) time.Duration {
	if snapshot.Terminal() {
		return ProgressFinalTTL
	}
	return ProgressRunningTTL
}
