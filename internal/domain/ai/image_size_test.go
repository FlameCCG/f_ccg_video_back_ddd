package ai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveImageSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		model      string
		size       string
		resolution string
		ratio      string
		want       string
	}{
		{name: "explicit size passthrough", model: testDoubaoSeedream3, size: "1024x768", want: "1024x768"},
		{name: "default resolution", model: testDoubaoSeedream3, want: "2K"},
		{name: "1k adaptive", model: testDoubaoSeedream3, resolution: "1k", ratio: "adaptive", want: "1K"},
		{name: "1k adaptive on seedream 4.5", model: testDoubaoSeedream45, resolution: "1k", want: "2K"},
		{name: "3k adaptive", model: testDoubaoSeedream3, resolution: "3k", want: "3K"},
		{name: "4k adaptive", model: testDoubaoSeedream3, resolution: "4k", want: "4K"},
		{name: "unknown resolution", model: testDoubaoSeedream3, resolution: "9k", want: "2K"},
		{name: "16:9 at 2k", model: testDoubaoSeedream3, resolution: "2k", ratio: test169, want: "2048x1152"},
		{name: "9:16 at 1k", model: testDoubaoSeedream3, resolution: "1k", ratio: "9:16", want: "576x1024"},
		{name: "1:1 at 2k", model: testDoubaoSeedream3, resolution: "2k", ratio: "1:1", want: "2048x2048"},
		{
			name:       "16:9 at 1k bumped for seedream 4.5",
			model:      testDoubaoSeedream45,
			resolution: "1k",
			ratio:      test169,
			want:       "2560x1440",
		},
		{name: "invalid ratio falls back to resolution keyword", model: "m", resolution: "2k", ratio: "5:4", want: "2K"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ResolveImageSize(tt.model, tt.size, tt.resolution, tt.ratio)
			require.Equal(t, tt.want, got, "ResolveImageSize(%q, %q, %q, %q) = %q, want %q", tt.model, tt.size, tt.resolution, tt.ratio, got, tt.want)
		})
	}
}

func TestResolveImageSizeSeedreamExplicitSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		model string
		size  string
		want  string
	}{
		{name: "keyword 1k upgraded", model: "doubao-seedream-5", size: "1k", want: "2K"},
		{name: "keyword 2k normalized", model: "doubao-seedream-5", size: "2k", want: "2K"},
		{name: "keyword 4k normalized", model: testDoubaoSeedream45, size: "4k", want: "4K"},
		{name: "large enough passthrough", model: testDoubaoSeedream45, size: "2048x2048", want: "2048x2048"},
		{name: "known ratio scaled up", model: testDoubaoSeedream45, size: "1024x576", want: "2560x1440"},
		{name: "unparsable passthrough", model: testDoubaoSeedream45, size: "big", want: "big"},
		{name: "non seedream passthrough", model: testDoubaoSeedream3, size: "16x16", want: "16x16"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			{
				got := ResolveImageSize(tt.model, tt.size, "", "")
				require.Equal(t, tt.want, got, "ResolveImageSize(%q, %q) = %q, want %q", tt.model, tt.size, got, tt.want)
			}
		})
	}
}

func TestResolveImageSizeSeedreamOddRatioScaling(t *testing.T) {
	t.Parallel()

	// 5:4 不在已知比例表内，走等比放大 + 8 的倍数对齐分支。
	got := ResolveImageSize("doubao-seedream-5", "500x400", "", "")

	width, height, ok := parseImageSize(got)
	require.True(t, ok, "ResolveImageSize returned unparsable size %q", got)

	require.Equal(t, 0, width%8, "scaled size %q is not aligned to 8", got)
	require.Equal(t, 0, height%8, "scaled size %q is not aligned to 8", got)

	require.GreaterOrEqual(t, width*height, minSeedreamImagePixels, "scaled size %q has %d pixels, want >= %d", got, width*height, minSeedreamImagePixels)
}

func TestGetSeedreamMinMaxSide(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ratio string
		want  int
	}{
		{ratio: "1:1", want: 2048},
		{ratio: test169, want: 2560},
		{ratio: "4:3", want: 2304},
		{ratio: "21:9", want: 3024},
		{ratio: "bogus", want: 2048},
	}

	for _, tt := range tests {
		t.Run(tt.ratio, func(t *testing.T) {
			t.Parallel()

			{
				got := getSeedreamMinMaxSide(tt.ratio)
				require.Equal(t, tt.want, got, "getSeedreamMinMaxSide(%q) = %d, want %d", tt.ratio, got, tt.want)
			}
		})
	}
}
