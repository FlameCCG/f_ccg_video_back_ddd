package storage

import (
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"
)

func TestDASHManifestReadsMinIOOnceAndListsSizesInOneBatch(t *testing.T) {
	t.Parallel()

	manifest := "<MPD/>"
	gets, lists := 0, 0
	transport := pipeTestTransport(func(req *http.Request) (*http.Response, error) {
		body := manifest

		if req.URL.Query().Get("list-type") == "2" {
			lists++

			require.Equal(t, "videos/dash/", req.URL.Query().Get("prefix"), "list prefix=%q", req.URL.Query().Get("prefix"))

			body = `<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>bucket</Name><IsTruncated>false</IsTruncated><Contents><Key>videos/dash/init.mp4</Key><Size>4</Size></Contents><Contents><Key>videos/dash/seg-1.m4s</Key><Size>7</Size></Contents></ListBucketResult>`
		} else {
			gets++

			require.Equal(t, http.MethodGet, req.Method, "unexpected request=%s %s", req.Method, req.URL)
			require.Equal(t, "/bucket/videos/dash/manifest.mpd", req.URL.Path, "unexpected request=%s %s", req.Method, req.URL)
		}

		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Length": []string{strconv.Itoa(len(body))}, "Last-Modified": []string{"Mon, 02 Jan 2006 15:04:05 GMT"}, "Etag": []string{testTest}}, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Request: req}, nil
	})

	client, err := minio.New("storage.example.com", &minio.Options{Transport: transport, Region: "us-east-1"})
	require.NoError(t, err)

	cfg := &site.Storage{MinIO: &site.MinIO{Enable: true, Endpoint: "storage.example.com", Bucket: "bucket", AccessKey: testTest, SecretKey: testTest}}
	pool := &MinIOClientPool{client: client, initKey: minioConfigKey(cfg.MinIO), logger: zap.NewNop()}
	provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{Storage: cfg}})

	data, sizes, err := NewResolver(provider, pool, nil, zap.NewNop()).ReadDASHManifest(t.Context(), "/videos/dash/manifest.mpd", 8<<20)
	require.NoError(t, err)

	require.Equal(t, string(data), manifest, "data=%s sizes=%v get=%d list=%d", data, sizes, gets, lists)
	require.EqualValues(t, 4, sizes["init.mp4"], "data=%s sizes=%v get=%d list=%d", data, sizes, gets, lists)
	require.EqualValues(t, 7, sizes["seg-1.m4s"], "data=%s sizes=%v get=%d list=%d", data, sizes, gets, lists)
	require.Equal(t, 1, gets, "data=%s sizes=%v get=%d list=%d", data, sizes, gets, lists)
	require.Equal(t, 1, lists, "data=%s sizes=%v get=%d list=%d", data, sizes, gets, lists)
}
