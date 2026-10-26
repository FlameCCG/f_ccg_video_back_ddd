package media

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildMP4SuperTargets(t *testing.T) {
	t.Parallel()

	task := TranscodeTask{VideoID: 7, FileHash: "hash", OutputPath: "video/7"}
	tests := []struct {
		name             string
		pixelHeight      uint16
		classHeight      uint16
		sourceResolution enum.VideoResolution
		resolutions      []uint16
		highBitrate      bool
		needsSource      bool
		wantIDs          []string
	}{
		{
			name:        "4k high bitrate includes source high and medium variants",
			pixelHeight: 2160, classHeight: 2160, sourceResolution: enum.Resolution2160P,
			resolutions: []uint16{360, 720}, highBitrate: true, needsSource: true,
			wantIDs: []string{
				"mp4:2160:source", "mp4:1080:high", "mp4:1080:medium",
				testMp4360Standard, testStandard720TargetID,
			},
		},
		{
			name:        "1080 high bitrate reuses source as high",
			pixelHeight: 1080, classHeight: 1080, sourceResolution: enum.Resolution1080P,
			resolutions: []uint16{360, 720}, highBitrate: true, needsSource: true,
			wantIDs: []string{"mp4:1080:source", "mp4:1080:medium", testMp4360Standard, testStandard720TargetID},
		},
		{
			name:        "normal 4k only creates configured derived outputs",
			pixelHeight: 2160, classHeight: 2160, sourceResolution: enum.Resolution2160P,
			resolutions: []uint16{360, 720, 1080},
			wantIDs:     []string{testMp4360Standard, testStandard720TargetID, "mp4:1080:standard"},
		},
		{
			name:        "ultrawide rejects 2160 ladder rung",
			pixelHeight: 1632, classHeight: 2160, sourceResolution: enum.Resolution2160P,
			resolutions: []uint16{360, 720, 1080, 2160},
			wantIDs:     []string{testMp4360Standard, testStandard720TargetID, "mp4:1080:standard"},
		},
		{
			name:        "playable non-standard 496 source reuses original 360 class",
			pixelHeight: 496, classHeight: 360, sourceResolution: enum.Resolution360P,
			resolutions: []uint16{360},
			wantIDs:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			targets := BuildMP4SuperTargets(
				task, tt.pixelHeight, tt.classHeight, tt.sourceResolution, tt.resolutions,
				tt.highBitrate, tt.needsSource,
			)

			got := make([]string, 0, len(targets))
			for _, target := range targets {
				got = append(got, target.ID)
			}

			require.Len(t, tt.wantIDs, len(got), "target count = %d, want %d: %v", len(got), len(tt.wantIDs), got)

			for i := range got {
				require.Equal(t, got[i], tt.wantIDs[i], "target[%d] = %q, want %q", i, got[i], tt.wantIDs[i])
			}
		})
	}
}

func TestBuildSourceNormalizeTargetsKeepsNonStandardHeight(t *testing.T) {
	t.Parallel()

	task := TranscodeTask{VideoID: 9, FileHash: testSrc, OutputPath: testVideoProgressKey}

	targets := BuildSourceNormalizeTargets(
		task,
		496,
		360,
		enum.Resolution360P,
		false,
	)
	require.Len(t, targets, 1, "want one source target, got %d", len(targets))

	target := targets[0]
	require.EqualValues(t, 496, target.Resolution, "resolution = %d, want 496", target.Resolution)

	require.True(t, target.KeepSource, "MP4 compatibility normalization must preserve the 496p source dimensions")

	require.Equal(t, enum.Resolution360P, target.DBResolution, "DB resolution = %q, want %q", target.DBResolution, enum.Resolution360P)
}

func TestNonStandardSourceUsesLowerResolutionLabel(t *testing.T) {
	t.Parallel()

	{
		got := MapResolutionEnum(864, 496)
		require.Equal(t, enum.Resolution360P, got, "MapResolutionEnum(864, 496) = %q, want %q", got, enum.Resolution360P)
	}
}

func TestBuildSourceNormalizeTargetsHighBitrate(t *testing.T) {
	t.Parallel()

	task := TranscodeTask{VideoID: 9, FileHash: testSrc, OutputPath: testVideoProgressKey}

	targets := BuildSourceNormalizeTargets(task, 1080, 1080, enum.Resolution1080P, true)
	require.Len(t, targets, 1, "want 1 source target, got %d", len(targets))

	require.Equal(t, QualityModeHigh, targets[0].Quality, "high-bitrate source: quality=%v maxrateUseSource=%v", targets[0].Quality, targets[0].MaxrateUseSource)
	require.False(t, targets[0].MaxrateUseSource, "high-bitrate source: quality=%v maxrateUseSource=%v", targets[0].Quality, targets[0].MaxrateUseSource)

	require.Equal(t, enum.Resolution1080PHigh, targets[0].DBResolution, "db resolution = %v, want 1080PHigh", targets[0].DBResolution)
}

func TestBuildSourceNormalizeTargetsUsesIsolatedObject(t *testing.T) {
	t.Parallel()

	task := TranscodeTask{
		VideoID:    9,
		FileHash:   testSrc,
		OutputPath: testVideoProgressKey,
		SourcePath: "http://localhost/videos/video/9/_source_src.mp4",
	}

	first := BuildSourceNormalizeTargets(task, 1080, 1080, enum.Resolution1080P, false)

	second := BuildSourceNormalizeTargets(task, 1080, 1080, enum.Resolution1080P, false)
	require.False(t, len(first) != 1 || len(second) != 1, "want one source target per build, got %d and %d", len(first), len(second))

	require.NotEqual(t, "_source_src.mp4", first[0].FileName, "normalized target reused source object name %q", first[0].FileName)

	require.True(t, strings.HasPrefix(first[0].FileName, "_normalized_src"), "normalized target name = %q, want isolated prefix", first[0].FileName)
	// 断点续传要求确定性命名：重试按名字匹配已提交/已入库的规范化产物并原地覆盖。
	require.Equal(t, first[0].RelativePath, second[0].RelativePath, "normalized target path must be deterministic across attempts: %q vs %q", first[0].RelativePath, second[0].RelativePath)

	{
		err := ValidateSourceTargetIsolation(task.SourcePath, first)
		require.NoError(t, err, "isolated target rejected: %v", err)
	}
}

func TestValidateSourceTargetIsolation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  TranscodeTarget
		wantErr bool
	}{
		{
			name: "reject source target with same object name",
			target: TranscodeTarget{
				IsSource:     true,
				RelativePath: "user_files/hash/_source_hash.mp4",
			},
			wantErr: true,
		},
		{
			name: "allow isolated normalized source target",
			target: TranscodeTarget{
				IsSource:     true,
				RelativePath: "user_files/hash/_normalized_hash_attempt.mp4",
			},
		},
		{
			name: "derived target is not the source replacement",
			target: TranscodeTarget{
				RelativePath: "user_files/hash/_source_hash.mp4",
			},
		},
	}

	const source = "http://localhost:9002/videos/user_files/hash/_source_hash.mp4"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSourceTargetIsolation(source, []TranscodeTarget{tt.target})
			require.Equal(t, tt.wantErr, (err != nil), "ValidateSourceTargetIsolation() error = %v, wantErr %v", err, tt.wantErr)
		})
	}
}

func TestBuildSourceNormalizeTargetsNonHighByHeight(t *testing.T) {
	t.Parallel()

	task := TranscodeTask{VideoID: 9, FileHash: testSrc, OutputPath: testVideoProgressKey}
	tests := []struct {
		height  uint16
		res     enum.VideoResolution
		quality QualityMode
	}{
		{2160, enum.Resolution2160P, QualityModeHigh},
		{1080, enum.Resolution1080P, QualityModeMedium},
		{720, enum.Resolution720P, QualityModeNone},
		{360, enum.Resolution360P, QualityModeNone},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("h%d_%s", tt.height, qualityModeName(tt.quality)), func(t *testing.T) {
			t.Parallel()

			targets := BuildSourceNormalizeTargets(task, tt.height, tt.height, tt.res, false)
			require.Len(t, targets, 1, "want 1 target, got %d", len(targets))

			require.Equal(t, targets[0].Quality, tt.quality, "quality = %v, want %v", targets[0].Quality, tt.quality)

			require.True(t, targets[0].MaxrateUseSource, "want MaxrateUseSource=true for non-high source normalize")
		})
	}
}

func TestUniqueResolutions(t *testing.T) {
	t.Parallel()

	got := UniqueResolutions([]uint16{360, 720, 1080, 2160, 1080, 0})
	want := []uint16{360, 720, 1080, 2160}

	require.Len(t, got, len(want), "want %v, got %v", want, got)

	for i := range want {
		require.Equal(t, got[i], want[i], "want %v, got %v", want, got)
	}
}

// TestBuildDashTargetsHighBitrateLadder 覆盖 DASH representation 构造：
// 原实现内联在 runDASHPipeline 的 run 闭包中，无独立测试；提取为领域函数后补齐。
func TestBuildDashTargetsHighBitrateLadder(t *testing.T) {
	t.Parallel()

	// 高码率 4K 源：顶档标记 dash:2160:high，1080 阶梯档为 medium，并追加 1080 high。
	targets := BuildDashTargets([]uint16{360, 720, 1080, 2160}, 2160, 2160, true)

	ids := make([]string, 0, len(targets))
	for _, target := range targets {
		ids = append(ids, target.ID)
	}

	joined := strings.Join(ids, ",")
	for _, want := range []string{"dash:2160:high", "dash:1080:medium", "dash:1080:high"} {
		require.Contains(t, joined, want, "missing %q in %v", want, ids)
	}

	// 按宽度归为 4K 的超宽源使用 2160 档位标签，但实际保持源像素。
	ultrawide := BuildDashTargets([]uint16{1080, 2160}, 1632, 2160, false)

	var nativeTop *TranscodeTarget

	for i := range ultrawide {
		if ultrawide[i].Resolution == 2160 {
			nativeTop = &ultrawide[i]
		}
	}

	require.NotNil(t, nativeTop, "native top representation missing: %+v", ultrawide)

	require.True(t, nativeTop.KeepSource, "native top must keep source and use remux maxrate: %+v", *nativeTop)
	require.True(t, nativeTop.MaxrateUseSource, "native top must keep source and use remux maxrate: %+v", *nativeTop)

	// 非 4K 分类的同高源向下归档，仍禁止添加高于真实像素的 representation。
	non4K := BuildDashTargets([]uint16{1080, 2160}, 1632, 1080, false)
	for _, target := range non4K {
		require.LessOrEqual(t, target.Resolution, uint16(1632), "must not upscale beyond pixel height: %+v", target)
	}
}

func qualityModeName(quality QualityMode) string {
	switch quality {
	case QualityModeHigh:
		return "high"
	case QualityModeMedium:
		return "medium"
	case QualityModeNone:
		return "standard"
	default:
		return "standard"
	}
}
