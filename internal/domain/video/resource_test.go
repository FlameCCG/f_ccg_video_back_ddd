package video_test

import (
	"testing"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
)

func TestDedupeResourceBriefsPrefersSourceAtSameResolution(t *testing.T) {
	t.Parallel()

	resources := []domain.ResourceBrief{
		{
			ID: 1, Resolution: enum.Resolution360P, FileURL: "derived-360.mp4",
			FileSize: 300, Bitrate: 600,
		},
		{
			ID: 2, Resolution: enum.Resolution360P, FileURL: "source-360.mp4",
			FileSize: 200, Bitrate: 500, IsSource: true,
		},
		{ID: 3, Resolution: enum.Resolution1080P, FileURL: "1080.mp4"},
		{ID: 4, Resolution: enum.Resolution1080PHigh, FileURL: "1080-high.mp4"},
		{ID: 5, Resolution: enum.ResolutionAuto, FileURL: "manifest.mpd"},
	}

	got := domain.DedupeResourceBriefs(resources)
	if len(got) != 4 {
		t.Fatalf("deduped resource count = %d, want 4: %#v", len(got), got)
	}

	for _, resource := range got {
		if resource.Resolution != enum.Resolution360P {
			continue
		}

		if !resource.IsSource || resource.FileURL != "source-360.mp4" {
			t.Fatalf("360p resource = %#v, want source resource", resource)
		}

		return
	}

	t.Fatal("missing 360p resource")
}
