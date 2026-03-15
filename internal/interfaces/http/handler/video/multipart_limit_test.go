package video

import (
	app "f_ccg_video_back_ddd/internal/application/video"
	"github.com/gin-gonic/gin"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

type boundedUploadSource struct{ left, read int64 }

func (r *boundedUploadSource) Read(p []byte) (int, error) {
	if r.left == 0 {
		return 0, io.EOF
	}
	n := len(p)
	if int64(n) > r.left {
		n = int(r.left)
	}
	clear(p[:n])
	r.left -= int64(n)
	r.read += int64(n)
	return n, nil
}

type multipartResolver struct{ app.StorageResolver }

func (multipartResolver) ValidateUploadID(string) error { return nil }

func TestCoverUploadStopsReadingOversizedMultipartBeforeStorage(t *testing.T) {
	payload := &boundedUploadSource{left: 64 << 20}
	prefix := "--audit-boundary\r\nContent-Disposition: form-data; name=\"fileHash\"\r\n\r\nabc\r\n--audit-boundary\r\nContent-Disposition: form-data; name=\"cover\"; filename=\"image.png\"\r\nContent-Type: image/png\r\n\r\n"
	req := httptest.NewRequest("POST", "/cover", io.MultiReader(strings.NewReader(prefix), payload, strings.NewReader("\r\n--audit-boundary--\r\n")))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=audit-boundary")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req
	h := &Handler{uploads: app.NewUploadService(multipartResolver{}, nil, nil, nil, nil)}
	h.UploadImageView(c)
	if payload.read > 12<<20 {
		t.Fatalf("read past body limit: %d", payload.read)
	}
	if payload.left == 0 {
		t.Fatal("consumed entire oversized body")
	}
	if c.Request.MultipartForm != nil {
		_ = c.Request.MultipartForm.RemoveAll()
	}
}
