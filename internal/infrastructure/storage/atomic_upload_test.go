package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/pkg/uploadscope"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type brokenUploadReader struct{ sent bool }

func (r *brokenUploadReader) Read(p []byte) (int, error) {
	if r.sent {
		return 0, errors.New("injected read failure")
	}

	r.sent = true

	return copy(p, "partial"), nil
}

func TestAtomicWritesPreservePreviousFileOnFailure(t *testing.T) {
	t.Parallel()

	//nolint:paralleltest // Cases share parent fixtures or must finish before their parent resource cleanup.
	for _, transcode := range []bool{false, true} {
		t.Run(strconv.FormatBool(transcode), func(t *testing.T) {
			root := t.TempDir()

			path := filepath.Join(root, "saved.mp4")
			{
				err := os.WriteFile(path, []byte("previous"), 0o600)
				require.NoError(t, err)
			}

			driver := NewLocalStorage(&site.Storage{Local: &site.Local{Path: root}}, nil)

			var err error
			if transcode {
				_, _, err = driver.SaveTranscodeVideo(t.Context(), "saved.mp4", 10, &brokenUploadReader{}, "video/mp4", testTest)
			} else {
				_, err = driver.SaveFile(t.Context(), "saved.mp4", 10, &brokenUploadReader{}, "video/mp4")
			}

			require.Error(t, err, "expected copy failure")

			got, err := os.ReadFile(path) // #nosec G304 -- Reads a fixture created inside this test's private temporary directory.
			require.NoError(t, err, "previous content lost: %q %v", got, err)
			require.Equal(t, "previous", string(got), "previous content lost: %q %v", got, err)

			entries, err := os.ReadDir(root)
			require.NoError(t, err, "temporary files leaked: %v %v", entries, err)
			require.Len(t, entries, 1, "temporary files leaked: %v %v", entries, err)
		})
	}
}

func TestConcurrentChunkRetriesCommitWholeFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	driver := NewLocalStorage(&site.Storage{ChunkDir: root}, nil)

	const size = 128 * 1024

	a, b := bytes.Repeat([]byte("a"), size), bytes.Repeat([]byte("b"), size)

	var wg sync.WaitGroup
	for i := range 12 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			payload := a
			if i%2 == 0 {
				payload = b
			}

			{
				err := driver.SaveChunk(context.Background(), testTest, "0", size, bytes.NewReader(payload))
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	got, err := os.ReadFile(filepath.Join(root, testTest, "0.part")) // #nosec G304 -- Reads a fixture created inside this test's private temporary directory.
	require.NoError(t, err)

	require.False(t, !bytes.Equal(got, a) && !bytes.Equal(got, b), "concurrent retries produced mixed content")
}

func TestUserChunkNamespacesAndVerifiedMerge(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	driver := NewLocalStorage(&site.Storage{ChunkDir: filepath.Join(root, "chunks"), Local: &site.Local{Path: filepath.Join(root, "stored")}}, nil)
	good := []byte("verified source")
	sum := sha256.Sum256(good)
	hash := hex.EncodeToString(sum[:])
	victim := uploadscope.WithOwner(t.Context(), 1)

	attacker := uploadscope.WithOwner(t.Context(), 2)
	for _, item := range []struct {
		ctx   context.Context //nolint:containedctx // Captures the request context to verify cancellation and ownership propagation.
		index string
		data  []byte
	}{{victim, "0", good}, {attacker, "0", []byte("poison")}, {attacker, "1234", []byte("extra")}} {
		{
			err := driver.SaveChunk(item.ctx, hash, item.index, int64(len(item.data)), bytes.NewReader(item.data))
			require.NoError(t, err)
		}
	}

	chunks, err := driver.ListUploadedChunks(victim, hash)
	require.NoError(t, err, "foreign chunks visible: %v %v", chunks, err)
	require.Len(t, chunks, 1, "foreign chunks visible: %v %v", chunks, err)
	require.Equal(t, "0", chunks[0], "foreign chunks visible: %v %v", chunks, err)

	{
		_, err := driver.MergeChunks(attacker, hash, "source.mp4", 1)
		require.Error(t, err, "mismatched content accepted")
	}

	{
		_, ok, err := driver.GetCompletedFile(victim, hash)
		require.NoError(t, err, "bad content published: %v %v", ok, err)
		require.False(t, ok, "bad content published: %v %v", ok, err)
	}

	{
		_, err := driver.MergeChunks(victim, hash, "source.mp4", 1)
		require.NoError(t, err)
	}

	path := filepath.Join(root, "stored", testVideos, hash, "_source_"+hash+".mp4")

	data, err := os.ReadFile(path) // #nosec G304 -- Reads a fixture created inside this test's private temporary directory.
	require.NoError(t, err, "wrong final content %q %v", data, err)
	require.True(t, bytes.Equal(data, good), "wrong final content %q %v", data, err)
}

var _ io.Reader = (*brokenUploadReader)(nil)
