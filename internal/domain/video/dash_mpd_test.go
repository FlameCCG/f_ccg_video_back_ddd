package video_test

import (
	"strings"
	"testing"

	domain "f_ccg_video_back_ddd/internal/domain/video"
)

// TestParseMPDVariants_VideoAndAudio 覆盖 MPD 解析：视频按高度降序、音频在最后。
func TestParseMPDVariants_VideoAndAudio(t *testing.T) {
	t.Parallel()

	xmlBody := `<?xml version="1.0" encoding="utf-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static">
  <Period id="0">
    <AdaptationSet id="0" contentType="video" frameRate="30/1">
      <Representation id="0" mimeType="video/mp4" codecs="avc1.64001e" bandwidth="157304" width="640" height="360">
        <BaseURL>hash-stream0.mp4</BaseURL>
        <SegmentList>
          <Initialization range="0-857" />
          <SegmentURL mediaRange="858-48993" />
          <SegmentURL mediaRange="48994-97353" />
        </SegmentList>
      </Representation>
      <Representation id="1" mimeType="video/mp4" codecs="avc1.64001f" bandwidth="336546" width="1280" height="720">
        <BaseURL>hash-stream1.mp4</BaseURL>
        <SegmentList>
          <Initialization range="0-855" />
          <SegmentURL mediaRange="856-174214" />
          <SegmentURL mediaRange="174215-351485" />
        </SegmentList>
      </Representation>
    </AdaptationSet>
    <AdaptationSet id="1" contentType="audio">
      <Representation id="2" mimeType="audio/mp4" codecs="mp4a.40.2" bandwidth="128000">
        <BaseURL>hash-audio.mp4</BaseURL>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`

	variants, err := domain.ParseMPDVariants([]byte(xmlBody))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(variants) != 3 {
		t.Fatalf("want 3 variants, got %d", len(variants))
	}

	// video sorted by height desc: 720 then 360, then audio
	if variants[0].Height != 720 || !strings.Contains(variants[0].Resolution, "720") {
		t.Fatalf("first should be 720p: %+v", variants[0])
	}

	if variants[0].Bitrate < 300 || variants[0].Bitrate > 400 {
		t.Fatalf("720p bitrate kbps unexpected: %d", variants[0].Bitrate)
	}

	if variants[0].FileName != "hash-stream1.mp4" {
		t.Fatalf("filename: %s", variants[0].FileName)
	}

	// last segment end 351485 → size 351486
	if variants[0].FileSize != 351486 {
		t.Fatalf("file size estimate: %d", variants[0].FileSize)
	}

	if variants[1].Height != 360 {
		t.Fatalf("second should be 360p: %+v", variants[1])
	}

	if variants[2].ContentType != "audio" || variants[2].Resolution != "音频" {
		t.Fatalf("audio variant: %+v", variants[2])
	}
}

// TestResolutionLabelFromSize 覆盖分辨率标签推导。
func TestResolutionLabelFromSize(t *testing.T) {
	t.Parallel()

	if !strings.Contains(domain.ResolutionLabelFromSize(1280, 720), "720") {
		t.Fatal(domain.ResolutionLabelFromSize(1280, 720))
	}

	if !strings.Contains(domain.ResolutionLabelFromSize(640, 360), "360") {
		t.Fatal(domain.ResolutionLabelFromSize(640, 360))
	}
}

// TestIsDASHResource 覆盖 DASH 资源判定（format=dash 或 .mpd 后缀）。
func TestIsDASHResource(t *testing.T) {
	t.Parallel()

	if !domain.IsDASHResource(domain.ResourceBrief{Format: "dash"}) {
		t.Fatal("format dash")
	}

	if !domain.IsDASHResource(domain.ResourceBrief{FileURL: "http://x/a/manifest.mpd"}) {
		t.Fatal("mpd suffix")
	}

	if domain.IsDASHResource(domain.ResourceBrief{Format: "mp4", FileURL: "http://x/a.mp4"}) {
		t.Fatal("mp4 should not match")
	}
}
