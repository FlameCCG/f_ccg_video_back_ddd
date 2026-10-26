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

	"github.com/stretchr/testify/require"

	media "f_ccg_video_back_ddd/internal/domain/media"

	"go.uber.org/zap"
)

func TestDASHArgsGenerateNativeTimelines(t *testing.T) {
	t.Parallel()

	policy := &media.TranscodePolicy{DashSegDuration: 4}

	targets := []media.TranscodeTarget{{MapLabel: "v0"}, {MapLabel: "v1"}}
	for _, gpu := range []bool{false, true} {
		args := buildDASHFFmpegArgs(policy, "source.mp4", "filter", targets, "manifest.mpd", "hash", gpu, 4000, "h264", true)
		assertNativeDASHArgs(t, args)

		options := dashTestOptions(args)
		for _, stream := range []string{"0", "1"} {
			require.Equal(t, "expr:gte(t,n_forced*4)", options["-force_key_frames:v:"+stream], "missing keyframe boundary for stream %s", stream)

			require.False(t, gpu && options["-forced-idr:v:"+stream] != "1", "NVENC stream %s is missing forced IDR", stream)

			require.False(t, !gpu && options["-forced-idr:v:"+stream] != "", "CPU encoder received NVENC-only option")
		}
	}

	assertNativeDASHArgs(t, buildChunkedDASHMuxArgs([]string{"rep0.txt", "rep1.txt"}, "audio.m4a", "manifest.mpd", "hash", 4))
}

func assertNativeDASHArgs(t *testing.T, args []string) {
	t.Helper()

	options := dashTestOptions(args)
	require.Equal(t, "1", options["-use_template"], "missing native timeline packaging options: %v", args)
	require.Equal(t, "1", options["-use_timeline"], "missing native timeline packaging options: %v", args)
	require.Equal(t, "hash-init-stream$RepresentationID$.mp4", options["-init_seg_name"], "missing native timeline packaging options: %v", args)
	require.Equal(t, "hash-stream$RepresentationID$-$Number%05d$.m4s", options["-media_seg_name"], "missing native timeline packaging options: %v", args)

	{
		_, exists := options["-single_file"]
		require.False(t, exists, "fixed-duration single-file packaging is still enabled")
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
	t.Parallel()

	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s unavailable: %v", tool, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	dir := t.TempDir()
	source := filepath.Join(dir, "source.mp4")
	runDASHFixtureFFmpeg(ctx, t, dir, []string{
		ffmpegHideBanner, ffmpegLogLevelFlag, "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=size=320x180:rate=30000/1001",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100",
		"-t", "13.2", "-c:v", "libx264", "-preset", "ultrafast",
		"-g", "250", "-sc_threshold", "0", "-c:a", "aac", source,
	})

	policy := &media.TranscodePolicy{DashSegDuration: 4, CPUPreset: "ultrafast"}
	targets := []media.TranscodeTarget{{Resolution: 180, KeepSource: true}, {Resolution: 90}}
	filter := buildFilterComplex(targets, false, false)

	//nolint:paralleltest // Cases share parent fixtures or must finish before their parent resource cleanup.
	for _, mode := range []string{"direct", "copy"} {
		t.Run(mode, func(t *testing.T) {
			outputDir := filepath.Join(dir, mode)
			{
				err := os.Mkdir(outputDir, 0o700)
				require.NoError(t, err)
			}

			mpdPath := filepath.Join(outputDir, media.DashManifestName)

			var args []string
			if mode == "direct" {
				args = buildDASHFFmpegArgs(policy, source, filter, targets, mpdPath, "hash", false, 0, "h264", true)
			} else {
				list, err := writeChunkConcatList(outputDir, "chunks.txt", []string{source})
				require.NoError(t, err)
				// 源片 GOP 刻意不对齐 4 秒，验证原生清单仍记录真实时间，而非假定固定段长。
				args = buildChunkedDASHMuxArgs([]string{list, list}, source, mpdPath, "hash", 4)
			}

			runDASHFixtureFFmpeg(ctx, t, outputDir, args)

			before, err := os.ReadFile(mpdPath) // #nosec G304 -- Reads a fixture created inside this test's private temporary directory.
			require.NoError(t, err)

			require.Contains(t, string(before), "<SegmentTimeline>", "FFmpeg did not directly produce a native timeline")
			require.NotContains(t, string(before), "<SegmentList", "FFmpeg did not directly produce a native timeline")

			{
				err := validateDASHOutput(ctx, outputDir, 13, zap.NewNop())
				require.NoError(t, err, "original FFmpeg output failed validation: %v", err)
			}

			after, _ := os.ReadFile(mpdPath) // #nosec G304 -- Reads a fixture created inside this test's private temporary directory.
			require.Equal(t, string(before), string(after), "validation rewrote the manifest")

			var manifest dashManifestXML
			{
				err := xml.Unmarshal(before, &manifest)
				require.NoError(t, err)
			}

			for _, set := range manifest.Periods[0].AdaptationSets {
				for _, rep := range set.Representations {
					require.NotNil(t, rep.Template, "representation missing native timeline")
					require.NotNil(t, rep.Template.Timeline, "representation missing native timeline")
				}
			}
		})
	}
}

func runDASHFixtureFFmpeg(ctx context.Context, t *testing.T, dir string, args []string) {
	t.Helper()

	cmd := exec.CommandContext(ctx, "ffmpeg", args...) // #nosec G204 -- Executes a fixed ffmpeg/ffprobe binary directly, with generated argv and no shell expansion.

	cmd.Dir = dir
	{
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "ffmpeg: %v\n%s", err, output)
	}
}

const testNativeDASHManifest = `<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static" mediaPresentationDuration="PT10S">
<Period><AdaptationSet contentType="video"><Representation id="0" width="320" height="180">
<SegmentTemplate timescale="1000" initialization="init-$RepresentationID$.mp4" media="seg-$RepresentationID$-$Number%05d$.m4s" startNumber="1">
<SegmentTimeline><S t="0" d="4000" r="1"/><S d="2000"/></SegmentTimeline>
</SegmentTemplate></Representation></AdaptationSet></Period></MPD>`

func TestInspectNativeDASHIsReadOnlyAndRejectsMissingSegments(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeDASHFixtureFile(t, dir, media.DashManifestName, testNativeDASHManifest)

	for _, file := range []string{"init-0.mp4", "seg-0-00001.m4s", "seg-0-00002.m4s", "seg-0-00003.m4s"} {
		writeDASHFixtureFile(t, dir, file, "media")
	}

	inspection, err := inspectDASHManifest(dir)
	require.NoError(t, err, "native manifest inspection = %+v, error = %v", inspection, err)
	require.Equal(t, inspection.videoPath, filepath.Join(dir, media.DashManifestName), "native manifest inspection = %+v, error = %v", inspection, err)

	{
		err := os.Remove(filepath.Join(dir, "seg-0-00002.m4s"))
		require.NoError(t, err)
	}

	{
		_, err := inspectDASHManifest(dir)
		require.Error(t, err, "missing media segment accepted")
	}

	data, _ := os.ReadFile(filepath.Join(dir, media.DashManifestName)) // #nosec G304 -- Reads a fixture created inside this test's private temporary directory.
	require.Equal(t, testNativeDASHManifest, string(data), "validation modified XML")
}

func TestInspectNativeDASHRejectsIncorrectTimingAndUnsafeReferences(t *testing.T) {
	t.Parallel()

	//nolint:paralleltest // Cases share parent fixtures or must finish before their parent resource cleanup.
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

			{
				_, err := inspectDASHManifest(dir)
				require.Error(t, err, "invalid manifest accepted")
			}
		})
	}
}
