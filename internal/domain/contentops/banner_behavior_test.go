package contentops_test

import (
	"errors"
	"testing"
	"time"

	contentopsdomain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

func TestBannerPlacementChangesAreAtomic(t *testing.T) {
	banner := contentopsdomain.RestoreBanner(contentopsdomain.BannerSnapshot{Type: enum.BannerTypeHome, PartitionID: 7})
	kind, partitionID := enum.BannerTypeUser, uint(3)
	before := banner.Snapshot()
	if err := banner.ChangePlacement(&kind, &partitionID); !errors.Is(err, contentopsdomain.ErrBannerPartitionTypeUnsupported) || banner.Snapshot() != before {
		t.Fatalf("invalid placement changed banner: banner=%+v err=%v", banner.Snapshot(), err)
	}
	if err := banner.ChangePlacement(&kind, nil); err != nil || banner.PartitionID() != 0 || banner.Type() != kind || banner.NeedsPartitionCheck() {
		t.Fatalf("switching type must remove old partition: banner=%+v err=%v", banner.Snapshot(), err)
	}
}

func TestNewBannerKeepsStateWhenPlacementFails(t *testing.T) {
	banner := contentopsdomain.NewBanner(" cover ", "href", false)
	if banner.ID() != 0 || !banner.CreatedAt().IsZero() || banner.Cover() != " cover " || banner.Href() != "href" || banner.Show() ||
		banner.Type() != enum.BannerTypeHome || banner.PartitionID() != 0 || banner.NeedsPartitionCheck() {
		t.Fatalf("constructor added restrictions: %+v", banner.Snapshot())
	}

	kind, partitionID := enum.BannerTypeTop, uint(3)
	before := banner.Snapshot()
	err := banner.ChangePlacement(&kind, &partitionID)
	if !errors.Is(err, contentopsdomain.ErrBannerPartitionTypeUnsupported) || banner.Snapshot() != before {
		t.Fatalf("failed placement changed new banner: banner=%+v err=%v", banner.Snapshot(), err)
	}
}

func TestRestoreBannerKeepsHistoricalPlacement(t *testing.T) {
	created := time.Date(2020, 5, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC)
	snapshot := contentopsdomain.BannerSnapshot{
		ID:          3,
		CreatedAt:   created,
		UpdatedAt:   updated,
		Cover:       "c",
		Href:        "h",
		Show:        false,
		Type:        enum.BannerTypeTop,
		PartitionID: 8,
	}

	banner := contentopsdomain.RestoreBanner(snapshot)
	if banner.Snapshot() != snapshot {
		t.Fatalf("restore revalidated historical state: %+v", banner.Snapshot())
	}

	kind := enum.BannerTypeHome
	partitionID := uint(8)
	if err := banner.ChangePlacement(&kind, &partitionID); err != nil || banner.Type() != kind || banner.PartitionID() != 8 || !banner.NeedsPartitionCheck() {
		t.Fatalf("related placement fields must change together: banner=%+v err=%v", banner.Snapshot(), err)
	}
	if !banner.CreatedAt().Equal(created) || banner.Cover() != "c" {
		t.Fatalf("placement changed unrelated fields: %+v", banner.Snapshot())
	}
}

func TestBannerSnapshotDoesNotAlias(t *testing.T) {
	created := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	banner := contentopsdomain.RestoreBanner(contentopsdomain.BannerSnapshot{
		ID:          4,
		CreatedAt:   created,
		Cover:       "a",
		Type:        enum.BannerTypeHome,
		PartitionID: 7,
	})

	snapshot := banner.Snapshot()
	snapshot.Cover = "b"
	snapshot.CreatedAt = created.Add(time.Hour)
	snapshot.PartitionID = 1
	if banner.Cover() != "a" || !banner.CreatedAt().Equal(created) || banner.PartitionID() != 7 {
		t.Fatalf("snapshot mutation leaked into banner: %+v", banner.Snapshot())
	}

	kind := enum.BannerTypeUser
	if err := banner.ChangePlacement(&kind, nil); err != nil {
		t.Fatalf("change placement: %v", err)
	}
	if snapshot.Type != enum.BannerTypeHome || snapshot.PartitionID != 1 || snapshot.Cover != "b" || !snapshot.CreatedAt.Equal(created.Add(time.Hour)) {
		t.Fatalf("banner mutation leaked into snapshot: %+v", snapshot)
	}
}

func TestVisibleAndAdminQueryPlacement(t *testing.T) {
	partitionID := uint(3)
	if _, _, err := (*contentopsdomain.RestoreBanner(contentopsdomain.BannerSnapshot{})).VisibleQuery(2, &partitionID); !errors.Is(err, contentopsdomain.ErrBannerPartitionFilterUnsupported) {
		t.Fatalf("visible filter err=%v", err)
	}
	if _, err := (*contentopsdomain.RestoreBanner(contentopsdomain.BannerSnapshot{})).AdminQueryType(2, &partitionID); !errors.Is(err, contentopsdomain.ErrBannerPartitionFilterUnsupported) {
		t.Fatalf("admin filter err=%v", err)
	}

	bannerType, resolved, err := (*contentopsdomain.RestoreBanner(contentopsdomain.BannerSnapshot{})).VisibleQuery(0, &partitionID)
	if err != nil || bannerType != enum.BannerTypeHome || resolved == nil || *resolved != 3 || resolved != &partitionID {
		t.Fatalf("partition-only visible query: type=%d partition=%v err=%v", bannerType, resolved, err)
	}

	bannerType, resolved, err = (*contentopsdomain.RestoreBanner(contentopsdomain.BannerSnapshot{})).VisibleQuery(1, nil)
	if err != nil || bannerType != enum.BannerTypeHome || resolved == nil || *resolved != 0 {
		t.Fatalf("home fallback: type=%d partition=%v err=%v", bannerType, resolved, err)
	}

	bannerType, resolved, err = (*contentopsdomain.RestoreBanner(contentopsdomain.BannerSnapshot{})).VisibleQuery(int(enum.BannerTypeUser), nil)
	if err != nil || bannerType != enum.BannerTypeUser || resolved != nil {
		t.Fatalf("user query should ignore partition: type=%d partition=%v err=%v", bannerType, resolved, err)
	}

	adminType, err := (*contentopsdomain.RestoreBanner(contentopsdomain.BannerSnapshot{})).AdminQueryType(0, &partitionID)
	if err != nil || adminType != enum.BannerTypeHome {
		t.Fatalf("admin partition-only type: type=%d err=%v", adminType, err)
	}
}
