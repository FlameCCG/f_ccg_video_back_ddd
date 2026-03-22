package video

import (
	"errors"
	"testing"
)

func TestUploadSizesPreserveErrorPriorityAndReplacementAccounting(t *testing.T) {
	const mb int64 = 1024 * 1024
	p := UploadSizePolicy{MaxChunkMB: 2, ChunkMB: 1, FileMB: 3}
	for _, tc := range []struct {
		size    int64
		message string
	}{{-1, "分块大小无效"}, {2*mb + 1, "分块大小超过限制（最大 2MB)"}, {4 * mb, "分块大小超过限制（最大 2MB)"}} {
		if err := p.IncomingChunk(tc.size); err == nil || err.Error() != tc.message {
			t.Errorf("size=%d err=%v", tc.size, err)
		}
	}
	if err := p.ReplacingChunk(3*mb, mb, mb); err != nil {
		t.Fatalf("same-size retry counted twice: %v", err)
	}
	if err := p.ReplacingChunk(3*mb, mb, 2*mb); err == nil || err.Error() != "文件大小超过限制（最大 3MB)" {
		t.Fatalf("aggregate limit changed: %v", err)
	}
	if err := p.ReplacingChunk(mb, 2*mb, mb); err != nil {
		t.Fatalf("historical oversized current chunk should clamp to total: %v", err)
	}
	if err := (UploadSizePolicy{ChunkMB: 1}).IncomingChunk(2 * mb); err == nil {
		t.Fatal("chunkSize fallback omitted")
	}
	if err := (UploadSizePolicy{}).IncomingChunk(100 * mb); err != nil {
		t.Fatal("unconfigured size limits must remain unlimited")
	}
	if limit := (UploadPolicy{}).DailyLimit(0); limit != 10 {
		t.Fatalf("default quota=%d", limit)
	}
	err := (UploadPolicy{}).EnsureDailyClaim(false, 10)
	if !errors.Is(err, ErrDailyUploadLimit) || err.Error() != "今日上传源视频文件数量已达上限，请明天再试（每日最多 10 个不同文件）" {
		t.Fatalf("quota error contract=%v", err)
	}
}
