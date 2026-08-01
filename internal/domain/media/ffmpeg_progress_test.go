package media

import (
	"testing"
)

func TestParseFFmpegProgressLine(t *testing.T) {
	t.Parallel()

	sec, ok := ParseFFmpegProgressLine("out_time_ms=12500")
	if !ok || sec < 12.4 || sec > 12.6 {
		t.Fatalf("out_time_ms got %v ok=%v", sec, ok)
	}

	sec, ok = ParseFFmpegProgressLine("out_time=00:01:02.500000")
	if !ok || sec < 62.4 || sec > 62.6 {
		t.Fatalf("out_time clock got %v ok=%v", sec, ok)
	}

	if _, ok := ParseFFmpegProgressLine("frame=1"); ok {
		t.Fatal("expected non-progress line to fail")
	}
}

func TestParseFFmpegTimeEquals(t *testing.T) {
	t.Parallel()

	sec, ok := ParseFFmpegTimeEquals("frame= 10 fps=25 time=00:00:05.00 bitrate=1000kbits/s")
	if !ok || sec < 4.9 || sec > 5.1 {
		t.Fatalf("got %v ok=%v", sec, ok)
	}
}

func TestInjectFFmpegProgressArgs(t *testing.T) {
	t.Parallel()

	in := []string{"ffmpeg", "-hide_banner", "-i", "a.mp4"}

	out := InjectFFmpegProgressArgs(in)
	if len(out) < 3 || out[0] != "ffmpeg" || out[1] != "-progress" || out[2] != "pipe:2" {
		t.Fatalf("unexpected args: %#v", out)
	}

	// idempotent
	out2 := InjectFFmpegProgressArgs(out)
	if len(out2) != len(out) {
		t.Fatalf("should not inject twice: %#v", out2)
	}
}

func TestTranscodeProgressKey(t *testing.T) {
	t.Parallel()

	if TranscodeProgressKey(3, 0) == TranscodeProgressKey(3, 1) {
		t.Fatal("part key should differ")
	}
}
