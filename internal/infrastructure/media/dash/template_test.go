package dash_test

import (
	"reflect"
	"testing"

	"f_ccg_video_back_ddd/internal/infrastructure/media/dash"
)

func TestNamespacedTimelineTemplateAndDirectory(t *testing.T) {
	body := `<d:MPD xmlns:d="urn:mpeg:dash:schema:mpd:2011"><d:Period><d:AdaptationSet contentType="video"><d:SegmentTemplate initialization="init-$RepresentationID$.mp4" media="seg-$Number%03d$-$Time$.m4s" startNumber="4"><d:SegmentTimeline><d:S t="10" d="5" r="1"/></d:SegmentTimeline></d:SegmentTemplate><d:Representation id="v" bandwidth="1000000" width="1920" height="1080"/></d:AdaptationSet></d:Period></d:MPD>`
	variants, err := dash.ParseMPDVariants([]byte(body))
	if err != nil || len(variants) != 1 {
		t.Fatalf("variants=%v err=%v", variants, err)
	}
	if !reflect.DeepEqual(variants[0].Files, []string{"init-v.mp4", "seg-004-10.m4s", "seg-005-15.m4s"}) || variants[0].Resolution != "1080p 高清" {
		t.Fatalf("%+v", variants[0])
	}
	if got := dash.DashDirPrefix("https://host/videos/8/dash/manifest.mpd?token=x"); got != "videos/8/dash/" {
		t.Fatal(got)
	}
}
