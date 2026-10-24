package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/pkg/uploadscope"
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
	for _, transcode := range []bool{false, true} {
		t.Run(fmt.Sprint(transcode), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "saved.mp4")
			if err := os.WriteFile(path, []byte("previous"), 0600); err != nil {
				t.Fatal(err)
			}
			driver := NewLocalStorage(&site.Storage{Local: &site.Local{Path: root}}, nil)
			var err error
			if transcode {
				_, _, err = driver.SaveTranscodeVideo(t.Context(), "saved.mp4", 10, &brokenUploadReader{}, "video/mp4", "test")
			} else {
				_, err = driver.SaveFile(t.Context(), "saved.mp4", 10, &brokenUploadReader{}, "video/mp4")
			}
			if err == nil {
				t.Fatal("expected copy failure")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != "previous" {
				t.Fatalf("previous content lost: %q %v", got, err)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary files leaked: %v %v", entries, err)
			}
		})
	}
}

func TestConcurrentChunkRetriesCommitWholeFiles(t *testing.T) {
	root := t.TempDir()
	driver := NewLocalStorage(&site.Storage{ChunkDir: root}, nil)
	const size = 128 * 1024
	a, b := bytes.Repeat([]byte("a"), size), bytes.Repeat([]byte("b"), size)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			payload := a
			if i%2 == 0 {
				payload = b
			}
			if err := driver.SaveChunk(context.Background(), "test", "0", size, bytes.NewReader(payload)); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	got, err := os.ReadFile(filepath.Join(root, "test", "0.part"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, a) && !bytes.Equal(got, b) {
		t.Fatal("concurrent retries produced mixed content")
	}
}

func TestUserChunkNamespacesAndVerifiedMerge(t *testing.T) {
	root := t.TempDir()
	driver := NewLocalStorage(&site.Storage{ChunkDir: filepath.Join(root, "chunks"), Local: &site.Local{Path: filepath.Join(root, "stored")}}, nil)
	good := []byte("verified source")
	sum := sha256.Sum256(good)
	hash := fmt.Sprintf("%x", sum)
	victim := uploadscope.WithOwner(t.Context(), 1)
	attacker := uploadscope.WithOwner(t.Context(), 2)
	for _, item := range []struct {
		ctx   context.Context
		index string
		data  []byte
	}{{victim, "0", good}, {attacker, "0", []byte("poison")}, {attacker, "1234", []byte("extra")}} {
		if err := driver.SaveChunk(item.ctx, hash, item.index, int64(len(item.data)), bytes.NewReader(item.data)); err != nil {
			t.Fatal(err)
		}
	}
	chunks, err := driver.ListUploadedChunks(victim, hash)
	if err != nil || len(chunks) != 1 || chunks[0] != "0" {
		t.Fatalf("foreign chunks visible: %v %v", chunks, err)
	}
	if _, err := driver.MergeChunks(attacker, hash, "source.mp4", 1); err == nil {
		t.Fatal("mismatched content accepted")
	}
	if _, ok, err := driver.GetCompletedFile(victim, hash); err != nil || ok {
		t.Fatalf("bad content published: %v %v", ok, err)
	}
	if _, err := driver.MergeChunks(victim, hash, "source.mp4", 1); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "stored", "videos", hash, "_source_"+hash+".mp4")
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, good) {
		t.Fatalf("wrong final content %q %v", data, err)
	}
}

var _ io.Reader = (*brokenUploadReader)(nil)
