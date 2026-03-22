package video_test

import (
	"errors"
	"testing"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	videodomain "f_ccg_video_back_ddd/internal/domain/video"
)

func TestReviewAndDuePublicationShareTimingRules(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	video, scheduledAt := videodomain.NewVideo(videodomain.VideoDraft{AuthorUsername: "author"}, future, false, true, now)
	if video.Status() != enum.VideoStatusReview || !video.IsScheduled() || scheduledAt == nil || !scheduledAt.Equal(future) {
		t.Fatalf("reviewed submission must retain schedule: video=%+v time=%v", video, scheduledAt)
	}
	if decision := video.PublishWhenDue(scheduledAt, future, true); decision.Published || video.Status() != enum.VideoStatusReview {
		t.Fatalf("scheduler cannot replace review: decision=%+v video=%+v", decision, video)
	}
	if changed, err := video.Review(enum.VideoStatusPublished, scheduledAt, now); err != nil || !changed || video.Status() != enum.VideoStatusScheduled || video.ApprovedAt() == nil || !video.ApprovedAt().Equal(now) {
		t.Fatalf("approval before schedule: changed=%v err=%v video=%+v", changed, err, video)
	}
	if decision := video.PublishWhenDue(scheduledAt, now, true); decision.Handled || video.Status() != enum.VideoStatusScheduled {
		t.Fatalf("future schedule must wait: decision=%+v video=%+v", decision, video)
	}
	if decision := video.PublishWhenDue(scheduledAt, future, true); !decision.Published || !decision.Handled || decision.RemovePlan || !video.IsPublished() {
		t.Fatalf("due publication must preserve schedule history: decision=%+v video=%+v", decision, video)
	}
	if decision := video.PublishWhenDue(scheduledAt, future, true); decision.Published || decision.RemovePlan || !decision.Handled {
		t.Fatalf("repeated publication must be idempotent: %+v", decision)
	}
}

func TestRejectedLifecycleActionsPreserveVideo(t *testing.T) {
	video := videodomain.RestoreVideo(videodomain.VideoSnapshot{AuthorUsername: "author", Status: enum.VideoStatusReview})
	before := *video
	if _, err := video.Review(enum.VideoStatusScheduled, nil, time.Time{}); !errors.Is(err, videodomain.ErrVideoStatusInvalid) || *video != before {
		t.Fatalf("invalid review mutated video: video=%+v err=%v", video, err)
	}
	if _, err := video.DeleteBy("other"); !errors.Is(err, videodomain.ErrVideoDeleteForbidden) || *video != before {
		t.Fatalf("unauthorized delete mutated video: video=%+v err=%v", video, err)
	}
	if changed, err := video.DeleteBy("author"); err != nil || !changed || !video.IsDeleted() {
		t.Fatalf("author delete: video=%+v changed=%v err=%v", video, changed, err)
	}
	if changed, err := video.DeleteBy("author"); err != nil || changed {
		t.Fatalf("repeated delete must be idempotent: changed=%v err=%v", changed, err)
	}
	if !video.Restore() || !video.IsPublished() || video.Restore() {
		t.Fatalf("restore must only change deleted state once: %+v", video)
	}
}
