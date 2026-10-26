package storage

import (
	"bytes"
	"errors"
	"f_ccg_video_back_ddd/internal/config/site"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type failingChunkReader struct{}

func (failingChunkReader) Read([]byte) (int, error) { return 0, errors.New("read interrupted") }

func TestDuplicateLocalChunkRefreshesLastSuccessfulWrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	driver := NewLocalStorage(&site.Storage{ChunkDir: root}, zap.NewNop())
	{
		err := driver.SaveChunk(t.Context(), "hash", "0", 1, bytes.NewReader([]byte("x")))
		require.NoError(t, err)
	}

	chunk := filepath.Join(root, "hash", "0.part")

	old := time.Now().Add(-3 * time.Hour)
	{
		err := os.Chtimes(chunk, old, old)
		require.NoError(t, err)
	}

	{
		err := driver.SaveChunk(t.Context(), "hash", "0", 1, bytes.NewReader([]byte("x")))
		require.NoError(t, err)
	}

	latest, err := latestLocalChunk(filepath.Join(root, "hash"))
	require.NoError(t, err)

	require.False(t, latest.Before(time.Now().Add(-time.Minute)), "duplicate chunk did not refresh last write: %v", latest)
}

func TestLocalMergeClosesSourceBeforeRename(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	driver := NewLocalStorage(&site.Storage{
		ChunkDir: filepath.Join(root, "chunks"), Local: &site.Local{Path: filepath.Join(root, "uploads")},
	}, zap.NewNop())
	{
		err := driver.SaveChunk(t.Context(), "hash", "0", 1, bytes.NewReader([]byte("x")))
		require.NoError(t, err)
	}

	path, err := driver.MergeChunks(t.Context(), "hash", "sample.mp4", 1)
	require.NoError(t, err, "merge path=%q error=%v", path, err)
	require.NotEmpty(t, path, "merge path=%q error=%v", path, err)

	_, completed, err := driver.GetCompletedFile(t.Context(), "hash")
	require.NoError(t, err, "completed=%v error=%v", completed, err)
	require.True(t, completed, "completed=%v error=%v", completed, err)
}

func TestLocalChunkUploadBytesSumsSavedParts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	driver := NewLocalStorage(&site.Storage{ChunkDir: root}, zap.NewNop())
	{
		err := driver.SaveChunk(t.Context(), "hash", "0", 1, bytes.NewReader([]byte("x")))
		require.NoError(t, err)
	}

	{
		err := driver.SaveChunk(t.Context(), "hash", "1", 2, bytes.NewReader([]byte("yz")))
		require.NoError(t, err)
	}

	total, current, err := driver.ChunkUploadBytes(t.Context(), "hash", "0")
	require.NoError(t, err)

	require.EqualValues(t, 3, total, "total=%d current=%d, want 3 and 1", total, current)
	require.EqualValues(t, 1, current, "total=%d current=%d, want 3 and 1", total, current)
}

func TestFailedLocalChunkRemovesTemporaryFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	driver := NewLocalStorage(&site.Storage{ChunkDir: root}, zap.NewNop())
	{
		err := driver.SaveChunk(t.Context(), "hash", "0", 1, failingChunkReader{})
		require.Error(t, err, "expected write error")
	}

	{
		_, err := os.Stat(filepath.Join(root, "hash", "0.part.uploading"))
		require.ErrorIs(t, err, os.ErrNotExist, "temporary file remains after failure: %v", err)
	}
}
