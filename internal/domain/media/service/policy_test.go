package service

import (
	"slices"
	"testing"

	mediavo "f_ccg_video_back_ddd/internal/domain/media/valueobject"
)

// testPolicy 是码率决策测试共用的策略。
func testPolicy() *mediavo.TranscodePolicy {
	return &mediavo.TranscodePolicy{
		Bitrate2160HighKbpsReduceRatio: 12,
		Bitrate1080HighKbpsReduceRatio: 15,
		Bitrate1080KbpsReduceRatio:     40,
		Bitrate720KbpsReduceRatio:      55,
	}
}

func TestHighQualityReduceRatio(t *testing.T) {
	t.Parallel()

	policy := testPolicy()

	if got := policy.HighQualityReduceRatio(2160); got != 12 {
		t.Fatalf("2160 high ratio = %d, want 12", got)
	}

	if got := policy.HighQualityReduceRatio(1080); got != 15 {
		t.Fatalf("1080 high ratio = %d, want 15", got)
	}
}

func TestOutputBitrateForTargetHighUsesConfig(t *testing.T) {
	t.Parallel()

	policy := testPolicy()

	src := 10000

	high1080 := policy.OutputBitrateForTarget(mediavo.TranscodeTarget{Quality: mediavo.QualityModeHigh, Resolution: 1080}, src, "h264")
	// High reduce 后若低于观感保底且源更高，抬到保底
	want1080High := mediavo.CalcReducedBitrate(src, 15)
	if floor := mediavo.ResolutionQualityFloorKbps(1080); want1080High < floor && src > floor {
		want1080High = floor
	}

	if high1080 != want1080High {
		t.Fatalf("1080 high maxrate = %d, want %d", high1080, want1080High)
	}

	high4k := policy.OutputBitrateForTarget(mediavo.TranscodeTarget{Quality: mediavo.QualityModeHigh, Resolution: 2160}, src, "h264")
	// reduce 后 8800 < 保底 12000，且源 10000 也 < 保底 → 退回源码率（不硬抬过源）
	want4kHigh := mediavo.CalcReducedBitrate(src, 12)

	if floor := mediavo.ResolutionQualityFloorKbps(2160); want4kHigh < floor {
		if src > floor {
			want4kHigh = floor
		} else {
			want4kHigh = src
		}
	}

	if high4k != want4kHigh {
		t.Fatalf("4k high maxrate = %d, want %d", high4k, want4kHigh)
	}

	medium := policy.OutputBitrateForTarget(mediavo.TranscodeTarget{Quality: mediavo.QualityModeMedium, Resolution: 1080}, src, "h264")
	if want := policy.CalcOutputBitrateForRes(src, 1080); medium != want {
		t.Fatalf("1080 medium maxrate = %d, want %d", medium, want)
	}

	// 高码率 4K DASH 顶档：QualityModeHigh + 2160 → High reduce（+ 保底）
	dash4kHigh := policy.OutputBitrateForTarget(mediavo.TranscodeTarget{
		ID: "dash:2160:high", Quality: mediavo.QualityModeHigh, Resolution: 2160,
	}, src, "h264")
	if dash4kHigh != want4kHigh {
		t.Fatalf("dash 4k high maxrate = %d, want %d", dash4kHigh, want4kHigh)
	}

	// 非高码率 + HEVC 源片：remux 预算 = max(源*1.7, 档位保底)，且 ≤3×源
	hevcSrc := 6000
	nonHighSrc := policy.OutputBitrateForTarget(mediavo.TranscodeTarget{
		ID: "mp4:1632:source", Quality: mediavo.QualityModeHigh, Resolution: 1632, MaxrateUseSource: true,
	}, hevcSrc, "hevc")

	wantRemux := mediavo.MaxrateForBrowserRemux(hevcSrc, 1632, "hevc")
	if nonHighSrc != wantRemux {
		t.Fatalf("hevc remux maxrate = %d, want %d", nonHighSrc, wantRemux)
	}

	if nonHighSrc <= hevcSrc {
		t.Fatalf("hevc remux should boost above source: got %d src %d", nonHighSrc, hevcSrc)
	}

	// 源码率已是 H264 且高于保底：MaxrateUseSource 仍可因保底略抬，但不低于源
	h264Top := policy.OutputBitrateForTarget(mediavo.TranscodeTarget{
		ID: "mp4:2160:source", Quality: mediavo.QualityModeHigh, Resolution: 2160, MaxrateUseSource: true,
	}, 15000, "h264")
	if h264Top < 15000 {
		t.Fatalf("h264 top maxrate = %d, want >= 15000", h264Top)
	}

	// 普通 4K 阶梯 1080 标准档仍用 reduce + 新保底
	ladderPolicy := testPolicy()
	ladderPolicy.Bitrate1080KbpsReduceRatio = 40
	ladder1080 := ladderPolicy.OutputBitrateForTarget(mediavo.TranscodeTarget{
		ID: "mp4:1080:standard", Quality: mediavo.QualityModeNone, Resolution: 1080,
	}, src, "h264")

	if want := ladderPolicy.CalcOutputBitrateForRes(src, 1080); ladder1080 != want {
		t.Fatalf("ladder 1080 maxrate = %d, want %d", ladder1080, want)
	}
}

func TestBuildDashResolutionsUsesPixelAndClassHeight(t *testing.T) {
	t.Parallel()

	policy := &mediavo.TranscodePolicy{TranscodeResolutions: []uint16{360, 720, 1080, 2160}}
	tests := []struct {
		name        string
		pixelHeight uint16
		classHeight uint16
		want        []uint16
	}{
		{name: "below minimum keeps native height", pixelHeight: 240, classHeight: 360, want: []uint16{240}},
		{name: "screenshot source 496 floors to 360", pixelHeight: 496, classHeight: 360, want: []uint16{360}},
		{name: "below 720 floors to 360", pixelHeight: 719, classHeight: 360, want: []uint16{360}},
		{name: "exact 720 keeps 720", pixelHeight: 720, classHeight: 720, want: []uint16{360, 720}},
		{name: "below 1080 floors to 720", pixelHeight: 1079, classHeight: 720, want: []uint16{360, 720}},
		{name: "exact 1080 keeps 1080", pixelHeight: 1080, classHeight: 1080, want: []uint16{360, 720, 1080}},
		{name: "non 4k 1632 floors to 1080", pixelHeight: 1632, classHeight: 1080, want: []uint16{360, 720, 1080}},
		{name: "cinematic 4k 1632 keeps 2160 class", pixelHeight: 1632, classHeight: 2160, want: []uint16{360, 720, 1080, 2160}},
		{name: "below 2160 floors to 1080", pixelHeight: 2159, classHeight: 1080, want: []uint16{360, 720, 1080}},
		{name: "exact 2160 keeps 2160", pixelHeight: 2160, classHeight: 2160, want: []uint16{360, 720, 1080, 2160}},
		{name: "above 2160 caps at 2160", pixelHeight: 2304, classHeight: 2160, want: []uint16{360, 720, 1080, 2160}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := policy.BuildDashResolutions(nil, tt.pixelHeight, tt.classHeight)
			if !slices.Equal(got, tt.want) {
				t.Fatalf(
					"BuildDashResolutions(nil, %d, %d) = %v, want %v",
					tt.pixelHeight,
					tt.classHeight,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestCinematic4KDASHKeepsSourcePixels(t *testing.T) {
	t.Parallel()

	const (
		pixelHeight = 1632
		classHeight = 2160
	)

	if !mediavo.KeepDASHSourcePixels(2160, pixelHeight, classHeight) {
		t.Fatal("2160 class for 1632-high cinematic 4K should keep source pixels")
	}

	targets := BuildDashTargets([]uint16{2160}, pixelHeight, classHeight, false)
	if len(targets) != 1 {
		t.Fatalf("target count = %d, want 1", len(targets))
	}

	if !targets[0].KeepSource || targets[0].Resolution != 2160 {
		t.Fatalf("cinematic 4K target = %+v, want 2160 label with source pixels", targets[0])
	}
}

func TestDashTopResolutionHeight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		pixelHeight uint16
		classHeight uint16
		want        uint16
	}{
		{name: "unknown source", want: 0},
		{name: "sub 360 source", pixelHeight: 240, classHeight: 360, want: 240},
		{name: "standard floor", pixelHeight: 1079, classHeight: 720, want: 720},
		{name: "cinematic 4k class", pixelHeight: 1632, classHeight: 2160, want: 2160},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := mediavo.DASHTopResolutionHeight(tt.pixelHeight, tt.classHeight); got != tt.want {
				t.Fatalf(
					"DASHTopResolutionHeight(%d, %d) = %d, want %d",
					tt.pixelHeight,
					tt.classHeight,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestEffectivePixelHeight(t *testing.T) {
	t.Parallel()

	if got := mediavo.EffectivePixelHeight(1632, 2160); got != 1632 {
		t.Fatalf("prefer meta height: got %d", got)
	}

	if got := mediavo.EffectivePixelHeight(0, 1080); got != 1080 {
		t.Fatalf("fallback class: got %d", got)
	}
}

func TestMaxrateForBrowserRemux(t *testing.T) {
	t.Parallel()

	// HEVC 6Mbps 顶档应抬到至少 12Mbps 保底（受 3× 源限制内）
	got := mediavo.MaxrateForBrowserRemux(6000, 1632, "hevc")
	if got < 10000 {
		t.Fatalf("hevc 6m remux too low: %d", got)
	}

	if got > 18000 {
		t.Fatalf("hevc 6m remux too high: %d", got)
	}

	// 源极低：不得无脑拉满 12Mbps，硬顶 3× 源
	tiny := mediavo.MaxrateForBrowserRemux(800, 1632, "hevc")
	if tiny != 800*3 {
		t.Fatalf("tiny source cap: got %d, want %d", tiny, 800*3)
	}
}

// TestPolicyNilReceiverFallbacks 验证策略为 nil 时各方法使用规定的配置缺失回落值。
func TestPolicyNilReceiverFallbacks(t *testing.T) {
	t.Parallel()

	var policy *mediavo.TranscodePolicy

	if got := policy.HighBitrateThresholdOrDefault(); got != 10000 {
		t.Fatalf("nil threshold = %d, want 10000", got)
	}

	if got := policy.SegDurationOrDefault(); got != 4 {
		t.Fatalf("nil segDur = %d, want 4", got)
	}

	if got := policy.MaxWorkersOrDefault(); got != 2 {
		t.Fatalf("nil maxWorkers = %d, want 2", got)
	}

	if got := policy.PresetOrDefault(true); got != "p4" {
		t.Fatalf("nil gpu preset = %q, want p4", got)
	}

	if got := policy.PresetOrDefault(false); got != "medium" {
		t.Fatalf("nil cpu preset = %q, want medium", got)
	}

	if got := policy.ThreadsOrZero(); got != 0 {
		t.Fatalf("nil threads = %d, want 0", got)
	}

	if policy.UseHardwareScale(true) {
		t.Fatal("nil policy must not enable hardware scale")
	}

	// 标准档 CRF/CQP 的回落值。
	if got := policy.OutputCQP(1080, false); got != 22 {
		t.Fatalf("nil crf720 = %d, want 22", got)
	}

	if got := policy.OutputCQP(360, true); got != 26 {
		t.Fatalf("nil gpuCQP360 = %d, want 26", got)
	}

	// 策略缺失时不给 -maxrate。
	if got := policy.CalcOutputBitrateForRes(8000, 1080); got != 0 {
		t.Fatalf("nil policy bitrate = %d, want 0", got)
	}

	if got := policy.OutputBitrateForTarget(mediavo.TranscodeTarget{Resolution: 1080}, 8000, "h264"); got != 0 {
		t.Fatalf("nil policy target bitrate = %d, want 0", got)
	}
}
