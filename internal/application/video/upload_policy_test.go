package video_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"

	"go.uber.org/zap"
)

type changingUploadConfig struct{ calls int }

func (c *changingUploadConfig) Get() *config.Config {
	c.calls++
	limit := 1
	if c.calls > 1 {
		limit = 99
	}
	return &config.Config{Site: &site.Site{Storage: &site.Storage{DailyUploadLimit: limit, MaxChunkSize: int64(limit)}}}
}

func TestUploadQuotaAndSizeShareOneConfiguration(t *testing.T) {
	cfg := &changingUploadConfig{}
	quota := app.NewUploadQuotaService(&quotaMemory{}, uploadRoles{}, cfg)
	s := app.NewUploadService(&stubResolver{driver: &stubDriver{}}, cfg, quota, nil, zap.NewNop())
	err := s.UploadChunkForUser(t.Context(), 7, "hash", "0", 2*1024*1024, bytes.NewReader(nil))
	if err == nil || !strings.Contains(err.Error(), "最大 1MB") || cfg.calls != 1 {
		t.Fatalf("reads=%d err=%v", cfg.calls, err)
	}
}

func TestMultipartUploadClaimsShareOneConfiguration(t *testing.T) {
	cfg := &changingUploadConfig{}
	quota := app.NewUploadQuotaService(&quotaMemory{}, uploadRoles{}, cfg)
	s := app.NewUploadService(&stubResolver{driver: &stubDriver{}}, cfg, quota, nil, zap.NewNop())
	err := s.WithPublishUploads(t.Context(), 7, []string{"first", "second"}, func(context.Context) error { return nil }, func(context.Context) error { t.Fatal("published over quota"); return nil })
	if !errors.Is(err, app.ErrDailyUploadLimit) || cfg.calls != 1 {
		t.Fatalf("reads=%d err=%v", cfg.calls, err)
	}
}
