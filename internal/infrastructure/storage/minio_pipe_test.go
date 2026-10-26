package storage

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type pipeTestTransport func(*http.Request) (*http.Response, error)

func (f pipeTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type observedChunkBody struct {
	remaining int
	closed    chan struct{}
	once      sync.Once
}

func (b *observedChunkBody) Read(p []byte) (int, error) {
	if b.remaining == 0 {
		return 0, io.EOF
	}

	n := min(len(p), b.remaining)
	b.remaining -= n

	return n, nil
}
func (b *observedChunkBody) Close() error { b.once.Do(func() { close(b.closed) }); return nil }
func TestMinIOStreamMergeStopsWriterWhenUploadFails(t *testing.T) {
	t.Parallel()

	sourceStarted := make(chan struct{})

	var startedOnce sync.Once

	body := &observedChunkBody{remaining: 33554432, closed: make(chan struct{})}
	transport := pipeTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet {
			startedOnce.Do(func() { close(sourceStarted) })
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Length": []string{"33554432"}, "Content-Type": []string{"application/octet-stream"}, "Etag": []string{testTest}, "Last-Modified": []string{"Mon, 02 Jan 2006 15:04:05 GMT"}}, Body: body, ContentLength: 33554432, Request: r}, nil
		}

		select {
		case <-sourceStarted:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}

		return &http.Response{StatusCode: http.StatusForbidden, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("<Error><Code>AccessDenied</Code><Message>injected failure</Message></Error>")), Request: r}, nil
	})

	client, err := minio.New("storage.example.com", &minio.Options{Transport: transport, Region: "us-east-1"})
	require.NoError(t, err)

	driver := &MinIOStorage{client: client, bucket: testVideos, chunkPrefix: "chunks", logger: zap.NewNop()}

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	{
		_, err := driver.streamMergeChunks(ctx, "final", "hash", "video.mp4", 1)
		require.Error(t, err, "expected upload failure")
	}

	select {
	case <-body.closed:
		require.NotEqual(t, 0, body.remaining, "test did not interrupt an active pipe writer")
	case <-time.After(time.Second):
		t.Fatal("source reader leaked after sink failure")
	}
}
