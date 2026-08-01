package video_test

import (
	"testing"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
)

// TestResolvePublishVideoStatus 覆盖投稿/改稿状态解析的全部分支。
//
// 原测试通过 global.SetConfig 切换「内容审核开关」；新架构把该开关上提为显式入参，
// 因此改为直接传参（断言与用例集合逐条保持一致）。
func TestResolvePublishVideoStatus(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)

	tests := []struct {
		name       string
		reviewOn   bool
		isPrivate  bool
		publishAt  *time.Time
		wantStatus enum.VideoStatus
	}{
		{
			name:       "private always private",
			reviewOn:   true,
			isPrivate:  true,
			publishAt:  &future,
			wantStatus: enum.VideoStatusPrivate,
		},
		{
			name:       "future without review is scheduled",
			reviewOn:   false,
			publishAt:  &future,
			wantStatus: enum.VideoStatusScheduled,
		},
		{
			name:       "future with review is review",
			reviewOn:   true,
			publishAt:  &future,
			wantStatus: enum.VideoStatusReview,
		},
		{
			name:       "immediate without review is published",
			reviewOn:   false,
			publishAt:  &now,
			wantStatus: enum.VideoStatusPublished,
		},
		{
			name:       "immediate with review is review",
			reviewOn:   true,
			publishAt:  &now,
			wantStatus: enum.VideoStatusReview,
		},
		{
			name:       "nil publish time without review is published",
			reviewOn:   false,
			publishAt:  nil,
			wantStatus: enum.VideoStatusPublished,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.ResolvePublishVideoStatus(now, tt.publishAt, tt.isPrivate, tt.reviewOn)
			if got != tt.wantStatus {
				t.Fatalf("got status %v, want %v", got, tt.wantStatus)
			}
		})
	}
}

// TestShouldScheduleVideoPublish 覆盖是否写入定时发布任务的判定。
func TestShouldScheduleVideoPublish(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)

	if domain.ShouldScheduleVideoPublish(now, now.Add(time.Hour), true) {
		t.Fatal("private should not schedule")
	}

	if !domain.ShouldScheduleVideoPublish(now, now.Add(time.Hour), false) {
		t.Fatal("future public should schedule")
	}

	if domain.ShouldScheduleVideoPublish(now, now, false) {
		t.Fatal("immediate public should not schedule")
	}
}
