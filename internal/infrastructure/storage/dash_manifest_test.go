package storage

import (
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"fmt"
	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDASHManifestReadsMinIOOnceAndListsSizesInOneBatch(t *testing.T) {
	manifest := "<MPD/>"
	gets, lists := 0, 0
	transport := pipeTestTransport(func(req *http.Request) (*http.Response, error) {
		body := manifest
		if req.URL.Query().Get("list-type") == "2" {
			lists++
			if req.URL.Query().Get("prefix") != "videos/dash/" {
				t.Fatalf("list prefix=%q", req.URL.Query().Get("prefix"))
			}
			body = `<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>bucket</Name><IsTruncated>false</IsTruncated><Contents><Key>videos/dash/init.mp4</Key><Size>4</Size></Contents><Contents><Key>videos/dash/seg-1.m4s</Key><Size>7</Size></Contents></ListBucketResult>`
		} else {
			gets++
			if req.Method != http.MethodGet || req.URL.Path != "/bucket/videos/dash/manifest.mpd" {
				t.Fatalf("unexpected request=%s %s", req.Method, req.URL)
			}
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Length": []string{fmt.Sprint(len(body))}, "Last-Modified": []string{"Mon, 02 Jan 2006 15:04:05 GMT"}, "Etag": []string{"test"}}, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Request: req}, nil
	})
	client, err := minio.New("storage.example.com", &minio.Options{Transport: transport, Region: "us-east-1"})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &site.Storage{MinIO: &site.MinIO{Enable: true, Endpoint: "storage.example.com", Bucket: "bucket", AccessKey: "test", SecretKey: "test"}}
	pool := &MinIOClientPool{client: client, initKey: minioConfigKey(cfg.MinIO), logger: zap.NewNop()}
	provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{Storage: cfg}})
	data, sizes, err := NewResolver(provider, pool, nil, zap.NewNop()).ReadDASHManifest(t.Context(), "/videos/dash/manifest.mpd", 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != manifest || sizes["init.mp4"] != 4 || sizes["seg-1.m4s"] != 7 || gets != 1 || lists != 1 {
		t.Fatalf("data=%s sizes=%v get=%d list=%d", data, sizes, gets, lists)
	}
}
