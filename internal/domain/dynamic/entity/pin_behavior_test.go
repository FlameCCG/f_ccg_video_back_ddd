package entity_test

import (
	"errors"
	"testing"
	"time"

	dynamicerrors "f_ccg_video_back_ddd/internal/domain/dynamic/domainerr"
	"f_ccg_video_back_ddd/internal/domain/dynamic/entity"
)

func TestDynamicPinProtectsOwnerAndTimestamp(t *testing.T) {
	d := &entity.Dynamic{UserID: 7}
	now := time.Now()
	if err := d.SetPinnedBy(8, true, now); !errors.Is(err, dynamicerrors.ErrPinForbidden) || d.IsPinned || d.PinnedAt != nil {
		t.Fatalf("unauthorized pin: dynamic=%+v err=%v", d, err)
	}
	if err := d.SetPinnedBy(7, true, now); err != nil || d.PinnedAt == nil || !d.PinnedAt.Equal(now) {
		t.Fatalf("pin: dynamic=%+v err=%v", d, err)
	}
	if err := d.SetPinnedBy(7, false, now); err != nil || d.IsPinned || d.PinnedAt != nil {
		t.Fatalf("unpin: dynamic=%+v err=%v", d, err)
	}
}
