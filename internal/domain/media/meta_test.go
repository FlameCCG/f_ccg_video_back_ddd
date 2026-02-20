package media

import (
	"testing"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

func TestValidateTranscodedDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		sourceDuration int
		outputDuration int
		wantErr        bool
	}{
		{name: "matching duration", sourceDuration: 1388, outputDuration: 1388},
		{name: "small container drift", sourceDuration: 1388, outputDuration: 1370},
		{name: "truncated output", sourceDuration: 1388, outputDuration: 49, wantErr: true},
		{name: "missing output duration", sourceDuration: 1388, outputDuration: 0, wantErr: true},
		{name: "unknown source duration", sourceDuration: 0, outputDuration: 49},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateTranscodedDuration(tt.sourceDuration, tt.outputDuration)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateTranscodedDuration(%d, %d) error = %v, wantErr %v",
					tt.sourceDuration, tt.outputDuration, err, tt.wantErr)
			}
		})
	}
}

// TestVideoMetaIsHighBitrate 覆盖高码率判定：仅 1080P / 4K 档参与，阈值可配。
func TestVideoMetaIsHighBitrate(t *testing.T) {
	t.Parallel()

	fullHD := VideoMeta{Width: 1920, Height: 1080, Bitrate: 12000}
	if !fullHD.IsHighBitrate(10000) {
		t.Fatal("1080p 12Mbps must be high bitrate at threshold 10000")
	}

	if fullHD.IsHighBitrate(20000) {
		t.Fatal("1080p 12Mbps must not be high bitrate at threshold 20000")
	}

	// threshold <= 0 时用默认 10000。
	if !fullHD.IsHighBitrate(0) {
		t.Fatal("zero threshold must fall back to default 10000")
	}

	// 720P 及以下不参与判定。
	hd720 := VideoMeta{Width: 1280, Height: 720, Bitrate: 50000}
	if hd720.IsHighBitrate(10000) {
		t.Fatal("720p must never be high bitrate")
	}

	// 码率未知不判定。
	unknown := VideoMeta{Width: 1920, Height: 1080}
	if unknown.IsHighBitrate(10000) {
		t.Fatal("unknown bitrate must not be high bitrate")
	}
}

// TestVideoMetaAssessBrowserPlayableSource 覆盖「源能否被浏览器直播放」的各否决原因。
func TestVideoMetaAssessBrowserPlayableSource(t *testing.T) {
	t.Parallel()

	playable := VideoMeta{
		Format: "mov,mp4,m4a,3gp,3g2,mj2", MajorBrand: "isom", Codec: "h264",
		VideoProfile: "High", PixelFormat: "yuv420p", VideoLevel: 40,
		AudioCodec: "aac", AudioProfile: "LC",
	}

	if ok, reason := playable.AssessBrowserPlayableSource(); !ok {
		t.Fatalf("standard h264/aac mp4 must be playable, reason = %q", reason)
	}

	tests := []struct {
		name string
		meta VideoMeta
	}{
		{name: "quicktime brand", meta: withField(playable, func(m *VideoMeta) { m.MajorBrand = "qt  " })},
		{name: "hevc codec", meta: withField(playable, func(m *VideoMeta) { m.Codec = "hevc" })},
		{name: "exotic profile", meta: withField(playable, func(m *VideoMeta) { m.VideoProfile = "High 4:4:4 Predictive" })},
		{name: "10bit pixel format", meta: withField(playable, func(m *VideoMeta) { m.PixelFormat = "yuv420p10le" })},
		{name: "excessive level", meta: withField(playable, func(m *VideoMeta) { m.VideoLevel = 62 })},
		{name: "non aac audio", meta: withField(playable, func(m *VideoMeta) { m.AudioCodec = "opus" })},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ok, reason := tt.meta.AssessBrowserPlayableSource()
			if ok {
				t.Fatal("expected source to be rejected")
			}

			if reason == "" {
				t.Fatal("rejection must carry a reason")
			}
		})
	}
}

// TestVideoMetaCanonicalResourceFormat 浏览器友好容器统一记为 mp4，否则取首个 format token。
func TestVideoMetaCanonicalResourceFormat(t *testing.T) {
	t.Parallel()

	mp4 := VideoMeta{Format: "mov,mp4,m4a,3gp,3g2,mj2", MajorBrand: "isom"}
	if got := mp4.CanonicalResourceFormat(); got != "mp4" {
		t.Fatalf("mp4 container format = %q, want mp4", got)
	}

	mkv := VideoMeta{Format: "matroska,webm"}
	if got := mkv.CanonicalResourceFormat(); got != "matroska" {
		t.Fatalf("mkv container format = %q, want matroska", got)
	}

	empty := VideoMeta{}
	if got := empty.CanonicalResourceFormat(); got != "mp4" {
		t.Fatalf("empty format = %q, want mp4", got)
	}
}

// TestMapResolutionEnum 覆盖宽高 → 分辨率枚举映射。
func TestMapResolutionEnum(t *testing.T) {
	t.Parallel()

	tests := []struct {
		width, height int
		want          enum.VideoResolution
	}{
		{3840, 2160, enum.Resolution2160P},
		{1920, 1080, enum.Resolution1080P},
		{1280, 720, enum.Resolution720P},
		{640, 360, enum.Resolution360P},
		{320, 240, enum.Resolution360P},
	}

	for _, tt := range tests {
		if got := MapResolutionEnum(tt.width, tt.height); got != tt.want {
			t.Fatalf("MapResolutionEnum(%d,%d) = %q, want %q", tt.width, tt.height, got, tt.want)
		}
	}

	// 超宽片：宽 >= 3840 会被归到 2160 档，但真实像素高由 EffectivePixelHeight 保留。
	if got := MapResolutionEnum(3840, 1632); got != enum.Resolution2160P {
		t.Fatalf("ultrawide class = %q, want %q", got, enum.Resolution2160P)
	}

	// 按目标高度映射（派生阶梯落库用）。
	if got := MapResolutionEnumByHeight(1080); got != enum.Resolution1080P {
		t.Fatalf("MapResolutionEnumByHeight(1080) = %q", got)
	}

	if got := CanonicalResolutionHeight(enum.Resolution1080PHigh); got != 1080 {
		t.Fatalf("CanonicalResolutionHeight(1080High) = %d, want 1080", got)
	}
}

// withField 复制 meta 并施加一处改动，便于表驱动构造否决用例。
func withField(base VideoMeta, mutate func(*VideoMeta)) VideoMeta {
	out := base
	mutate(&out)

	return out
}
