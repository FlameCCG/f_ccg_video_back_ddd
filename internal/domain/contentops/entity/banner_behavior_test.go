package entity_test

import (
	"errors"
	"testing"

	contentopserrors "f_ccg_video_back_ddd/internal/domain/contentops/domainerr"
	"f_ccg_video_back_ddd/internal/domain/contentops/entity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

func TestBannerPlacementChangesAreAtomic(t *testing.T) {
	banner := &entity.Banner{Type: enum.BannerTypeHome, PartitionID: 7}
	kind, partitionID := enum.BannerTypeUser, uint(3)
	before := *banner
	if err := banner.ChangePlacement(&kind, &partitionID); !errors.Is(err, contentopserrors.ErrBannerPartitionTypeUnsupported) || *banner != before {
		t.Fatalf("invalid placement changed banner: banner=%+v err=%v", banner, err)
	}
	if err := banner.ChangePlacement(&kind, nil); err != nil || banner.PartitionID != 0 || banner.NeedsPartitionCheck() {
		t.Fatalf("switching type must remove old partition: banner=%+v err=%v", banner, err)
	}
}
