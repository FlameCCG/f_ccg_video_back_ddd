package runtime

import (
	"testing"
	"time"

	mediadomain "f_ccg_video_back_ddd/internal/domain/media"
)

func TestProgressRetentionPreservesExistingCacheLifetimes(t *testing.T) {
	for _, status := range []string{mediadomain.TranscodeStatusQueued, mediadomain.TranscodeStatusRunning, "", "unknown"} {
		if got := ProgressTTL(mediadomain.TranscodeProgress{Status: status}); got != 2*time.Hour {
			t.Fatalf("status %q TTL=%v, want 2h", status, got)
		}
	}
	for _, status := range []string{mediadomain.TranscodeStatusSucceeded, mediadomain.TranscodeStatusFailed} {
		if got := ProgressTTL(mediadomain.TranscodeProgress{Status: status}); got != 24*time.Hour {
			t.Fatalf("status %q TTL=%v, want 24h", status, got)
		}
	}
}
