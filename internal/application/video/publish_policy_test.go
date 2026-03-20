package video

import (
	"strings"
	"testing"

	"f_ccg_video_back_ddd/internal/config"

	"go.uber.org/zap"
)

type countingPublishConfig struct{ calls int }

func (c *countingPublishConfig) Get() *config.Config { c.calls++; return &config.Config{} }

func TestPublishCapturesConfigurationOnce(t *testing.T) {
	cfg := &countingPublishConfig{}
	s := NewPublishService(nil, nil, nil, nil, nil, nil, cfg, nil, zap.NewNop())
	_, err := s.PublishVideo(t.Context(), PublishVideoCommand{Parts: make([]PublishPartInput, 11)})
	if err == nil || !strings.Contains(err.Error(), "最多上传 10") || cfg.calls != 1 {
		t.Fatalf("calls=%d err=%v", cfg.calls, err)
	}
}
