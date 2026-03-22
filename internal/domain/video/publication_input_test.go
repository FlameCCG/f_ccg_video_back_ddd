package video

import (
	"errors"
	"strings"
	"testing"
)

func TestPublicationInputKeepsDefaultLimitAndErrorPriority(t *testing.T) {
	policy := UploadPolicy{}
	for _, maximum := range []int{0, -1, 10} {
		uploads := make([]PublishUpload, 11)
		if err := policy.EnsurePublishReady(uploads, true, maximum); err == nil || err.Error() != "单个作品最多上传 10 个分P" {
			t.Fatalf("count must be checked before missing source, maximum=%d err=%v", maximum, err)
		}
	}
	for _, multi := range []bool{false, true} {
		want := ErrUploadVideoFirst
		if multi {
			want = ErrUploadAllPartsFirst
		}
		if err := policy.EnsurePublishReady([]PublishUpload{{FileName: " ", FileHash: "hash"}}, multi, 3); !errors.Is(err, want) {
			t.Fatalf("missing upload, multi=%v: %v", multi, err)
		}
	}
	// 先校验输入完整性，再校验文件名长度。
	if err := policy.EnsurePublishReady([]PublishUpload{{FileName: strings.Repeat("字", 256), FileHash: "hash"}}, false, 0); err != nil {
		t.Fatal(err)
	}
}
