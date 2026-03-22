package dynamic_test

import (
	"errors"
	"testing"
	"time"

	dynamicdomain "f_ccg_video_back_ddd/internal/domain/dynamic"
)

func TestDynamicPinProtectsOwnerAndTimestamp(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	d := dynamicdomain.RestoreDynamic(dynamicdomain.DynamicSnapshot{UserID: 7, IsPinned: true, PinnedAt: &now})
	later := now.Add(time.Hour)
	if err := d.SetPinnedBy(8, false, later); !errors.Is(err, dynamicdomain.ErrPinForbidden) || !d.IsPinned() || d.PinnedAt() == nil || !d.PinnedAt().Equal(now) {
		t.Fatalf("unauthorized pin changed state: pinned=%v at=%v err=%v", d.IsPinned(), d.PinnedAt(), err)
	}
	if err := d.SetPinnedBy(7, true, later); err != nil || !d.IsPinned() || d.PinnedAt() == nil || !d.PinnedAt().Equal(later) {
		t.Fatalf("pin: pinned=%v at=%v err=%v", d.IsPinned(), d.PinnedAt(), err)
	}
	if err := d.SetPinnedBy(7, false, later); err != nil || d.IsPinned() || d.PinnedAt() != nil {
		t.Fatalf("unpin: pinned=%v at=%v err=%v", d.IsPinned(), d.PinnedAt(), err)
	}
}
