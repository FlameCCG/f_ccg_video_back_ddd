package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"f_ccg_video_back_ddd/internal/config/site"

	"go.uber.org/zap"
)

type failingChunkReader struct{}

func (failingChunkReader) Read([]byte) (int, error) { return 0, errors.New("read interrupted") }

func TestDuplicateLocalChunkRefreshesLastSuccessfulWrite(t *testing.T) {
	root := t.TempDir()
	driver := NewLocalStorage(&site.Storage{ChunkDir: root}, zap.NewNop())
	if err := driver.SaveChunk(t.Context(), "hash", "0", 1, bytes.NewReader([]byte("x"))); err != nil {
		t.Fatal(err)
	}
	chunk := filepath.Join(root, "hash", "0.part")
	old := time.Now().Add(-3 * time.Hour)
	if err := os.Chtimes(chunk, old, old); err != nil {
		t.Fatal(err)
	}
	if err := driver.SaveChunk(t.Context(), "hash", "0", 1, bytes.NewReader([]byte("x"))); err != nil {
		t.Fatal(err)
	}
	latest, err := latestLocalChunk(filepath.Join(root, "hash"))
	if err != nil {
		t.Fatal(err)
	}
	if latest.Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("duplicate chunk did not refresh last write: %v", latest)
	}
}

func TestLocalMergeClosesSourceBeforeRename(t *testing.T) {
	root := t.TempDir()
	driver := NewLocalStorage(&site.Storage{
		ChunkDir: filepath.Join(root, "chunks"), Local: &site.Local{Path: filepath.Join(root, "uploads")},
	}, zap.NewNop())
	if err := driver.SaveChunk(t.Context(), "hash", "0", 1, bytes.NewReader([]byte("x"))); err != nil {
		t.Fatal(err)
	}
	path, err := driver.MergeChunks(t.Context(), "hash", "sample.mp4", 1)
	if err != nil || path == "" {
		t.Fatalf("merge path=%q error=%v", path, err)
	}
	_, completed, err := driver.GetCompletedFile(t.Context(), "hash")
	if err != nil || !completed {
		t.Fatalf("completed=%v error=%v", completed, err)
	}
}

func TestFailedLocalChunkRemovesTemporaryFile(t *testing.T) {
	root := t.TempDir()
	driver := NewLocalStorage(&site.Storage{ChunkDir: root}, zap.NewNop())
	if err := driver.SaveChunk(t.Context(), "hash", "0", 1, failingChunkReader{}); err == nil {
		t.Fatal("expected write error")
	}
	if _, err := os.Stat(filepath.Join(root, "hash", "0.part.uploading")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file remains after failure: %v", err)
	}
}
