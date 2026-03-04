package video_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"go.uber.org/zap"
)

type uploadRoles struct {
	admin bool
	err   error
}

func (r uploadRoles) IsUploadQuotaExempt(context.Context, uint) (bool, error) {
	return r.admin, r.err
}

type quotaMemory struct {
	mu     sync.Mutex
	hashes map[uint]map[string]bool
	err    error
}

func (m *quotaMemory) Claim(_ context.Context, uid uint, hash string, limit int) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.hashes == nil {
		m.hashes = make(map[uint]map[string]bool)
	}
	if m.hashes[uid] == nil {
		m.hashes[uid] = make(map[string]bool)
	}
	if m.hashes[uid][hash] {
		return true, nil
	}
	if len(m.hashes[uid]) >= limit {
		return false, nil
	}
	m.hashes[uid][hash] = true
	return true, nil
}

func quotaUploadService(driver *stubDriver, store app.UploadQuotaStore, admin bool, limit int) *app.UploadService {
	provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{Storage: &site.Storage{DailyUploadLimit: limit}}})
	quota := app.NewUploadQuotaService(store, uploadRoles{admin: admin}, provider)
	return app.NewUploadService(&stubResolver{driver: driver}, provider, quota, nil, zap.NewNop())
}

func TestFirstChunkFailureStillConsumesQuota(t *testing.T) {
	driver := &stubDriver{saveErr: errors.New("disk full")}
	svc := quotaUploadService(driver, &quotaMemory{}, false, 1)
	if err := svc.UploadChunkForUser(t.Context(), 7, "first", "0", 1, bytes.NewReader([]byte("x"))); err == nil || err.Error() != "disk full" {
		t.Fatalf("first write failure = %v", err)
	}
	if err := svc.UploadChunkForUser(t.Context(), 7, "second", "0", 1, bytes.NewReader([]byte("x"))); !errors.Is(err, app.ErrDailyUploadLimit) {
		t.Fatalf("second file should hit quota, got %v", err)
	}
	driver.saveErr = nil
	if err := svc.UploadChunkForUser(t.Context(), 7, "first", "0", 1, bytes.NewReader([]byte("x"))); err != nil {
		t.Fatalf("same hash resume: %v", err)
	}
}

func TestInstantUploadClaimsOnlyCompletedAndIsIdempotent(t *testing.T) {
	driver := &stubDriver{completed: false}
	svc := quotaUploadService(driver, &quotaMemory{}, false, 1)
	status, err := svc.GetUploadStatusForUser(t.Context(), 7, "uncompleted")
	if err != nil || status.Completed {
		t.Fatalf("incomplete status: %+v, %v", status, err)
	}
	driver.completed, driver.completedFilePath = true, "/videos/hash/_source_hash.mp4"
	for i := 0; i < 2; i++ {
		status, err = svc.GetUploadStatusForUser(t.Context(), 7, "instant")
		if err != nil || !status.Completed {
			t.Fatalf("completed status %d: %+v, %v", i, status, err)
		}
	}
	if _, err := svc.GetUploadStatusForUser(t.Context(), 7, "other"); !errors.Is(err, app.ErrDailyUploadLimit) {
		t.Fatalf("next distinct completed hash = %v", err)
	}
}

func TestAdminBypassesDailyLimitAndRedisFailureRejectsUser(t *testing.T) {
	store := &quotaMemory{err: errors.New("redis down")}
	admin := quotaUploadService(&stubDriver{}, store, true, 1)
	for i := 0; i < 12; i++ {
		if err := admin.UploadChunkForUser(t.Context(), 1, fmt.Sprintf("hash%d", i), "0", 1, bytes.NewReader([]byte("x"))); err != nil {
			t.Fatalf("admin file %d: %v", i, err)
		}
	}
	user := quotaUploadService(&stubDriver{}, store, false, 1)
	if err := user.UploadChunkForUser(t.Context(), 2, "hash", "0", 1, bytes.NewReader([]byte("x"))); !errors.Is(err, app.ErrUploadQuotaUnavailable) {
		t.Fatalf("ordinary upload on Redis error = %v", err)
	}
}

func TestMergeAndMultipartPublishFallbackClaim(t *testing.T) {
	driver := &stubDriver{completed: true, completedFilePath: "/videos/hash/_source_hash.mp4"}
	svc := quotaUploadService(driver, &quotaMemory{}, false, 2)
	if _, err := svc.CompleteUploadForUser(t.Context(), 7, "first", "a.mp4", 1); err != nil {
		t.Fatal(err)
	}
	called := false
	if err := svc.WithPublishUploads(t.Context(), 7, []string{"first", "second", "second"}, func(context.Context) error { return nil }, func(context.Context) error {
		called = true
		return nil
	}); err != nil || !called {
		t.Fatalf("multipart publish: called=%v err=%v", called, err)
	}
	if err := svc.WithPublishUploads(t.Context(), 7, []string{"third"}, func(context.Context) error { return nil }, func(context.Context) error { return nil }); !errors.Is(err, app.ErrDailyUploadLimit) {
		t.Fatalf("third distinct file = %v", err)
	}
}

func TestPublishMissingSourceDoesNotClaim(t *testing.T) {
	svc := quotaUploadService(&stubDriver{}, &quotaMemory{}, false, 1)
	missing := errors.New("source missing")
	persisted := false
	err := svc.WithPublishUploads(t.Context(), 7, []string{"missing"},
		func(context.Context) error { return missing },
		func(context.Context) error { persisted = true; return nil })
	if !errors.Is(err, missing) || persisted {
		t.Fatalf("invalid publish: persisted=%v err=%v", persisted, err)
	}
	if err := svc.UploadChunkForUser(t.Context(), 7, "real", "0", 1, bytes.NewReader([]byte("x"))); err != nil {
		t.Fatalf("missing publish consumed quota: %v", err)
	}
}

func TestPublishKeepsPerWorkPartLimitSeparate(t *testing.T) {
	provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{Storage: &site.Storage{MaxUploadNum: 2, DailyUploadLimit: 10}}})
	svc := app.NewPublishService(nil, nil, nil, nil, nil, provider, nil, zap.NewNop())
	_, err := svc.PublishVideo(t.Context(), app.PublishVideoCommand{Parts: []app.PublishPartInput{{FileName: "a", FileHash: "a"}, {FileName: "b", FileHash: "b"}, {FileName: "c", FileHash: "c"}}})
	if err == nil || err.Error() != "单个作品最多上传 2 个分P" {
		t.Fatalf("per-work limit = %v", err)
	}
}
