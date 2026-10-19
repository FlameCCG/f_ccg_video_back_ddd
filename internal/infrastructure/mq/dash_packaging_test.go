package mq

import (
	"context"
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	media "f_ccg_video_back_ddd/internal/domain/media"

	"go.uber.org/zap"
)

func TestDASHArgsGenerateNativeTimelines(t *testing.T) {
	policy := &media.TranscodePolicy{DashSegDuration: 4}
	targets := []media.TranscodeTarget{{MapLabel: "v0"}, {MapLabel: "v1"}}
	for _, gpu := range []bool{false, true} {
		args := buildDASHFFmpegArgs(policy, "source.mp4", "filter", targets, "manifest.mpd", "hash", gpu, 4000, "h264", true)
		assertNativeDASHArgs(t, args)
		options := dashTestOptions(args)
		for _, stream := range []string{"0", "1"} {
			if options["-force_key_frames:v:"+stream] != "expr:gte(t,n_forced*4)" {
				t.Fatalf("missing keyframe boundary for stream %s", stream)
			}
			if gpu && options["-forced-idr:v:"+stream] != "1" {
				t.Fatalf("NVENC stream %s is missing forced IDR", stream)
			}
			if !gpu && options["-forced-idr:v:"+stream] != "" {
				t.Fatal("CPU encoder received NVENC-only option")
			}
		}
	}
	assertNativeDASHArgs(t, buildChunkedDASHMuxArgs([]string{"rep0.txt", "rep1.txt"}, "audio.m4a", "manifest.mpd", "hash", 4))
}

func assertNativeDASHArgs(t *testing.T, args []string) {
	t.Helper()
	options := dashTestOptions(args)
	if options["-use_template"] != "1" || options["-use_timeline"] != "1" ||
		options["-init_seg_name"] != "hash-init-stream$RepresentationID$.mp4" ||
		options["-media_seg_name"] != "hash-stream$RepresentationID$-$Number%05d$.m4s" {
		t.Fatalf("missing native timeline packaging options: %v", args)
	}
	if _, exists := options["-single_file"]; exists {
		t.Fatal("fixed-duration single-file packaging is still enabled")
	}
}

func dashTestOptions(args []string) map[string]string {
	options := make(map[string]string)
	for i := 0; i+1 < len(args); i++ {
		options[args[i]] = args[i+1]
	}
	return options
}

// 使用真实 FFmpeg 覆盖直接编码和断点块 copy 拼装，确认原始 MPD 不经改写即有效。
func TestDASHNativePackagingWithFFmpeg(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s unavailable: %v", tool, err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	dir := t.TempDir()
	source := filepath.Join(dir, "source.mp4")
	runDASHFixtureFFmpeg(t, ctx, dir, []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=size=320x180:rate=30000/1001",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100",
		"-t", "13.2", "-c:v", "libx264", "-preset", "ultrafast",
		"-g", "250", "-sc_threshold", "0", "-c:a", "aac", source,
	})
	policy := &media.TranscodePolicy{DashSegDuration: 4, CpuPreset: "ultrafast"}
	targets := []media.TranscodeTarget{{Resolution: 180, KeepSource: true}, {Resolution: 90}}
	filter := buildFilterComplex(targets, false, false)
	for _, mode := range []string{"direct", "copy"} {
		t.Run(mode, func(t *testing.T) {
			outputDir := filepath.Join(dir, mode)
			if err := os.Mkdir(outputDir, 0o700); err != nil {
				t.Fatal(err)
			}
			mpdPath := filepath.Join(outputDir, media.DashManifestName)
			var args []string
			if mode == "direct" {
				args = buildDASHFFmpegArgs(policy, source, filter, targets, mpdPath, "hash", false, 0, "h264", true)
			} else {
				list, err := writeChunkConcatList(outputDir, "chunks.txt", []string{source})
				if err != nil {
					t.Fatal(err)
				}
				// 源片 GOP 刻意不对齐 4 秒，验证原生清单仍记录真实时间，而非假定固定段长。
				args = buildChunkedDASHMuxArgs([]string{list, list}, source, mpdPath, "hash", 4)
			}
			runDASHFixtureFFmpeg(t, ctx, outputDir, args)
			before, err := os.ReadFile(mpdPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(before), "<SegmentTimeline>") || strings.Contains(string(before), "<SegmentList") {
				t.Fatal("FFmpeg did not directly produce a native timeline")
			}
			if err := validateDASHOutput(ctx, outputDir, 13, zap.NewNop()); err != nil {
				t.Fatalf("original FFmpeg output failed validation: %v", err)
			}
			after, _ := os.ReadFile(mpdPath)
			if string(before) != string(after) {
				t.Fatal("validation rewrote the manifest")
			}
			var manifest dashManifestXML
			if err := xml.Unmarshal(before, &manifest); err != nil {
				t.Fatal(err)
			}
			for _, set := range manifest.Periods[0].AdaptationSets {
				for _, rep := range set.Representations {
					if rep.Template == nil || rep.Template.Timeline == nil {
						t.Fatal("representation missing native timeline")
					}
				}
			}
		})
	}
}

func runDASHFixtureFFmpeg(t *testing.T, ctx context.Context, dir string, args []string) {
	t.Helper()
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v\n%s", err, output)
	}
}

const testNativeDASHManifest = `<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static" mediaPresentationDuration="PT10S">
<Period><AdaptationSet contentType="video"><Representation id="0" width="320" height="180">
<SegmentTemplate timescale="1000" initialization="init-$RepresentationID$.mp4" media="seg-$RepresentationID$-$Number%05d$.m4s" startNumber="1">
<SegmentTimeline><S t="0" d="4000" r="1"/><S d="2000"/></SegmentTimeline>
</SegmentTemplate></Representation></AdaptationSet></Period></MPD>`

func TestInspectNativeDASHIsReadOnlyAndRejectsMissingSegments(t *testing.T) {
	dir := t.TempDir()
	writeDASHFixtureFile(t, dir, media.DashManifestName, testNativeDASHManifest)
	for _, file := range []string{"init-0.mp4", "seg-0-00001.m4s", "seg-0-00002.m4s", "seg-0-00003.m4s"} {
		writeDASHFixtureFile(t, dir, file, "media")
	}
	inspection, err := inspectDASHManifest(dir)
	if err != nil || inspection.videoPath != filepath.Join(dir, media.DashManifestName) {
		t.Fatalf("native manifest inspection = %+v, error = %v", inspection, err)
	}
	if err := os.Remove(filepath.Join(dir, "seg-0-00002.m4s")); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectDASHManifest(dir); err == nil {
		t.Fatal("missing media segment accepted")
	}
	data, _ := os.ReadFile(filepath.Join(dir, media.DashManifestName))
	if string(data) != testNativeDASHManifest {
		t.Fatal("validation modified XML")
	}
}

func TestInspectNativeDASHRejectsIncorrectTimingAndUnsafeReferences(t *testing.T) {
	for _, test := range []struct{ name, old, replacement string }{
		{"short timeline", `d="4000" r="1"`, `d="1000" r="1"`},
		{"unbounded repeat", `r="1"`, `r="-1"`},
		{"timeline gap", `<S d="2000"/>`, `<S t="9000" d="2000"/>`},
		{"unsafe initialization", `init-$RepresentationID$.mp4`, `../init-$RepresentationID$.mp4`},
		{"external segments", `seg-$RepresentationID$-$Number%05d$.m4s`, `http://example.com/$Number$.m4s`},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDASHFixtureFile(t, dir, media.DashManifestName, strings.Replace(testNativeDASHManifest, test.old, test.replacement, 1))
			for _, file := range []string{"init-0.mp4", "seg-0-00001.m4s", "seg-0-00002.m4s", "seg-0-00003.m4s"} {
				writeDASHFixtureFile(t, dir, file, "media")
			}
			if _, err := inspectDASHManifest(dir); err == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
}
