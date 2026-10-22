package video_test

import (
	"errors"
	"testing"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
)

func TestReviseVisibility(t *testing.T) {
	now := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)
	future, past := now.Add(time.Hour), now.Add(-time.Hour)
	public, private := false, true
	for _, tt := range []struct {
		name    string
		from    enum.VideoStatus
		privacy *bool
		when    *time.Time
		review  bool
		want    enum.VideoStatus
	}{
		{"metadata preserves review", enum.VideoStatusReview, nil, nil, false, enum.VideoStatusReview},
		{"time only schedules public", enum.VideoStatusPublished, nil, &future, false, enum.VideoStatusScheduled},
		{"time only still needs review", enum.VideoStatusPublished, nil, &future, true, enum.VideoStatusReview},
		{"time only preserves privacy", enum.VideoStatusPrivate, nil, &future, false, enum.VideoStatusPrivate},
		{"make private", enum.VideoStatusPublished, &private, &future, false, enum.VideoStatusPrivate},
		{"publish private", enum.VideoStatusPrivate, &public, nil, false, enum.VideoStatusPublished},
		{"publish needs review", enum.VideoStatusPrivate, &public, nil, true, enum.VideoStatusReview},
		{"reschedule to past", enum.VideoStatusScheduled, nil, &past, false, enum.VideoStatusPublished},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := domain.Video{ID: 1, AuthorUsername: "alice", Status: tt.from, Views: 99}
			if err := v.Revise("alice", domain.Revision{IsPrivate: tt.privacy, PublishTime: tt.when}, now, tt.review); err != nil {
				t.Fatal(err)
			}
			if v.Status != tt.want || v.Views != 99 {
				t.Fatalf("status=%v, views=%d; want %v, 99", v.Status, v.Views, tt.want)
			}
		})
	}
}

func TestRevisePreservesPublicationTypeAndClearsApprovalWhenResubmitted(t *testing.T) {
	now := time.Now()
	approved, future := now.Add(-time.Hour), now.Add(time.Hour)
	v := &domain.Video{ID: 1, AuthorUsername: "alice", Status: enum.VideoStatusScheduled, IsScheduled: true, ApprovedAt: &approved}
	title := "new title"
	if err := v.Revise("alice", domain.Revision{Title: &title}, now, true); err != nil {
		t.Fatal(err)
	}
	if !v.IsScheduled || v.ApprovedAt == nil || !v.ApprovedAt.Equal(approved) {
		t.Fatalf("metadata revision lost publication info: %+v", v)
	}
	if err := v.Revise("alice", domain.Revision{PublishTime: &future}, now, true); err != nil {
		t.Fatal(err)
	}
	if !v.IsScheduled || v.Status != enum.VideoStatusReview || v.ApprovedAt != nil {
		t.Fatalf("resubmission retained stale approval: %+v", v)
	}
	private := true
	if err := v.Revise("alice", domain.Revision{IsPrivate: &private}, now, true); err != nil {
		t.Fatal(err)
	}
	if v.IsScheduled || v.Status != enum.VideoStatusPrivate {
		t.Fatalf("cancelled scheduling retained current publication type: %+v", v)
	}
}

func TestReviseRejectsInvalidAggregateWithoutMutation(t *testing.T) {
	title, public := "changed", false
	for _, tt := range []struct {
		name   string
		video  *domain.Video
		author string
		want   error
	}{
		{"missing", nil, "alice", domain.ErrRecordNotFound},
		{"deleted cannot revive", &domain.Video{AuthorUsername: "alice", Status: enum.VideoStatusDeleted}, "alice", domain.ErrRecordNotFound},
		{"different author", &domain.Video{AuthorUsername: "alice", Status: enum.VideoStatusPublished}, "bob", domain.ErrRecordNotFound},
		{"empty author", &domain.Video{Status: enum.VideoStatusPublished}, "", domain.ErrRecordNotFound},
		{"unknown state", &domain.Video{AuthorUsername: "alice", Status: 99}, "alice", domain.ErrVideoStatusInvalid},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var before domain.Video
			if tt.video != nil {
				before = *tt.video
			}
			err := tt.video.Revise(tt.author, domain.Revision{Title: &title, IsPrivate: &public}, time.Now(), false)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error=%v, want %v", err, tt.want)
			}
			if tt.video != nil && *tt.video != before {
				t.Fatal("rejected revision mutated video")
			}
		})
	}
}
