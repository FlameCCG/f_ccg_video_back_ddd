package video

import (
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"reflect"
	"testing"
	"time"
)

func TestReportHandlingKeepsAssociatedStateAtomicAndPendingOnly(t *testing.T) {
	at := time.Now()
	report := RestoreVideoReport(VideoReportSnapshot{ID: 1, Status: enum.ReportStatusPending, Reason: "historical"})
	before := report.Snapshot()
	if _, err := report.Handle(ReportHandling{}); !errors.Is(err, ErrReportStatusInvalid) || !reflect.DeepEqual(report.Snapshot(), before) {
		t.Fatal("invalid plan changed the report", err)
	}
	if _, err := report.PlanHandling(enum.ReportStatusPending, "note", 7, at); !errors.Is(err, ErrReportStatusInvalid) {
		t.Fatal(err)
	}
	plan, err := report.PlanHandling(enum.ReportStatusResolved, " done ", 7, at)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := report.Handle(plan)
	if err != nil || !changed || report.Status() != enum.ReportStatusResolved || report.HandleNote() != "done" || report.HandledBy() != 7 || !report.HandledAt().Equal(at) || report.Reason() != "historical" {
		t.Fatal(report.Snapshot(), err)
	}
	after := report.Snapshot()
	other, err := (DanmuReport{}).PlanHandling(enum.ReportStatusRejected, "reject", 0, at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := report.Handle(other); changed || err != nil || !reflect.DeepEqual(report.Snapshot(), after) {
		t.Fatal("handled report changed twice", err)
	}
	danmu := RestoreDanmuReport(DanmuReportSnapshot{ID: 2, Status: enum.ReportStatusPending})
	if changed, err := danmu.Handle(other); !changed || err != nil || danmu.Status() != enum.ReportStatusRejected || danmu.HandledBy() != 0 {
		t.Fatal(danmu.Snapshot(), err)
	}
	*danmu.HandledAt() = time.Time{}
	if !danmu.HandledAt().Equal(at.Add(time.Hour)) {
		t.Fatal("handling time escaped encapsulation")
	}
}

func TestRelatedSelectionPreservesSourceOrderCapacityAndCopies(t *testing.T) {
	selection := NewRelatedSelection(7, 2)
	selection.Add([]uint{7, 1, 1})
	if !selection.NeedsMore() || selection.FetchLimit() != 4 {
		t.Fatal(selection.IDs())
	}
	selection.Add([]uint{2, 3})
	selection.Add([]uint{4})
	if selection.NeedsMore() || !reflect.DeepEqual(selection.IDs(), []uint{1, 2}) {
		t.Fatal(selection.IDs())
	}
	selection.IDs()[0] = 99
	excluded := selection.ExcludedIDs()
	if len(excluded) != 3 || selection.IDs()[0] != 1 {
		t.Fatal("selection leaked mutable state")
	}
	if NewRelatedSelection(7, 0).FetchLimit() != 24 || NewRelatedSelection(7, 99).FetchLimit() != 100 {
		t.Fatal("recommendation defaults changed")
	}
}

func TestEffectivePublicationTimePreservesPrivateAndScheduledIntent(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	for _, tc := range []struct {
		requested time.Time
		private   bool
		want      time.Time
	}{
		{time.Time{}, false, now}, {now.Add(-time.Hour), false, now}, {future, true, now}, {future, false, future}, {now, false, now},
	} {
		if got := (Video{}).EffectivePublishTime(tc.requested, tc.private, now); !got.Equal(tc.want) {
			t.Fatal(got, tc.want)
		}
	}
	video, scheduled := NewVideo(VideoDraft{}, future, false, true, now)
	if video.Status() != enum.VideoStatusReview || !video.IsScheduled() || scheduled == nil || !scheduled.Equal(future) {
		t.Fatal("review and schedule no longer coexist")
	}
}
