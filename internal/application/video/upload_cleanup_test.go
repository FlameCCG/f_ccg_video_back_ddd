package video_test

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/internal/infrastructure/storage"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/video"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"go.uber.org/zap"
)

type cleanupReferences struct {
	domain.Repository
	paths map[string]bool
	err   error
}

func (r *cleanupReferences) StoragePathStillReferenced(_ context.Context, path string) (bool, error) {
	return r.paths[path], r.err
}

type cleanupGuard struct{ blocked map[string]bool }

func (g cleanupGuard) Run(ctx context.Context, _ []string, fn func(context.Context) error) error {
	return fn(ctx)
}

func (g cleanupGuard) TryRun(ctx context.Context, hash string, fn func(context.Context) error) (bool, error) {
	if g.blocked[hash] {
		return false, nil
	}

	return true, fn(ctx)
}

func oldFile(t *testing.T, path string, modified time.Time) {
	t.Helper()

	{
		err := os.MkdirAll(filepath.Dir(path), 0o750)
		require.NoError(t, err)
	}

	{
		err := os.WriteFile(path, []byte("source"), 0o600)
		require.NoError(t, err)
	}

	{
		err := os.Chtimes(path, modified, modified)
		require.NoError(t, err)
	}
}

func TestUploadCleanupScansLegacyFilesAndProtectsReferences(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	chunks, sources := filepath.Join(root, "chunks"), filepath.Join(root, "uploads")

	old := time.Now().Add(-3 * time.Hour)
	for _, hash := range []string{"unused", "active"} {
		oldFile(t, filepath.Join(chunks, hash, "0.part"), old)
	}

	for _, hash := range []string{"orphan", "referenced"} {
		oldFile(t, filepath.Join(sources, "videos", hash, "_source_"+hash+".mp4"), old)
	}

	recent := filepath.Join(sources, "videos", "recent", "_source_recent.mp4")
	oldFile(t, recent, time.Now())

	provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{Storage: &site.Storage{
		ChunkDir: chunks, Local: &site.Local{Path: sources},
		UnmergedChunkRetentionHours: 2, UnpublishedSourceRetentionHours: 1,
	}}})
	scanner := storage.NewUploadGarbageScanner(provider, nil, zap.NewNop())

	items, err := scanner.List(t.Context(), time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	require.NoError(t, err)

	require.Len(t, items, 4, "legacy candidates = %d, want 4: %+v", len(items), items)

	refs := &cleanupReferences{paths: map[string]bool{}}

	for _, item := range items {
		if item.FileHash == "referenced" {
			refs.paths[item.SourcePath] = true
		}
	}

	cleanup := app.NewUploadCleanupService(scanner, refs, cleanupGuard{blocked: map[string]bool{"active": true}}, provider, zap.NewNop())
	cleanup.Run(t.Context())

	for _, path := range []string{
		filepath.Join(chunks, "active", "0.part"),
		filepath.Join(sources, "videos", "referenced", "_source_referenced.mp4"), recent,
	} {
		{
			_, err := os.Stat(path)
			require.NoError(t, err, "protected file %s missing: %v", path, err)
		}
	}

	for _, path := range []string{
		filepath.Join(chunks, "unused", "0.part"),
		filepath.Join(sources, "videos", "orphan", "_source_orphan.mp4"),
	} {
		{
			_, err := os.Stat(path)
			require.ErrorIs(t, err, os.ErrNotExist, "orphan %s remains: %v", path, err)
		}
	}
}

func TestUploadCleanupSkipsQueryFailureAndFreshRewrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "videos", "hash", "_source_hash.mp4")
	oldFile(t, source, time.Now().Add(-2*time.Hour))

	provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{Storage: &site.Storage{Local: &site.Local{Path: root}}}})
	scanner := storage.NewUploadGarbageScanner(provider, nil, zap.NewNop())
	before := time.Now().Add(-time.Hour)

	items, err := scanner.List(t.Context(), before, before)
	require.NoError(t, err, "scan = %+v, %v", items, err)
	require.Len(t, items, 1, "scan = %+v, %v", items, err)

	refs := &cleanupReferences{err: errors.New("mysql unavailable")}
	app.NewUploadCleanupService(scanner, refs, cleanupGuard{}, provider, zap.NewNop()).Run(t.Context())

	{
		_, err := os.Stat(source)
		require.NoError(t, err, "query failure deleted source: %v", err)
	}

	refs.err = nil
	{
		err := os.Chtimes(source, time.Now(), time.Now())
		require.NoError(t, err)
	}

	deleted, err := scanner.DeleteIfExpired(t.Context(), items[0], before)
	require.NoError(t, err, "fresh rewrite deleted: %v, %v", deleted, err)
	require.False(t, deleted, "fresh rewrite deleted: %v, %v", deleted, err)
}
