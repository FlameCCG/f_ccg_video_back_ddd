package ai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func intPtr(v int) *int {
	return &v
}

func TestValidateImageGeneration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prompt  string
		n       *int
		wantErr error
	}{
		{name: "ok without n", prompt: testCatPrompt, wantErr: nil},
		{name: "ok with n", prompt: testCatPrompt, n: intPtr(4), wantErr: nil},
		{name: "empty prompt", prompt: "   ", wantErr: ErrEmptyPrompt},
		{name: "zero n", prompt: testCatPrompt, n: intPtr(0), wantErr: ErrImageCountRange},
		{name: "n too large", prompt: testCatPrompt, n: intPtr(16), wantErr: ErrImageCountRange},
		{name: "n at max", prompt: testCatPrompt, n: intPtr(15), wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			{
				err := ValidateImageGeneration(tt.prompt, tt.n)
				require.ErrorIs(t, err, tt.wantErr, "ValidateImageGeneration = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateImageEdit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		prompt    string
		hasImage  bool
		imagesLen int
		n         *int
		wantErr   error
	}{
		{name: "single image ok", prompt: testEditPrompt, hasImage: true, wantErr: nil},
		{name: "images ok", prompt: testEditPrompt, imagesLen: 3, n: intPtr(2), wantErr: nil},
		{name: "empty prompt", prompt: "", hasImage: true, wantErr: ErrEmptyPrompt},
		{name: "neither provided", prompt: testEditPrompt, wantErr: ErrImageRefExclusive},
		{name: "both provided", prompt: testEditPrompt, hasImage: true, imagesLen: 2, wantErr: ErrImageRefExclusive},
		{name: "too many refs", prompt: testEditPrompt, imagesLen: 15, wantErr: ErrTooManyImageRefs},
		{name: "n out of range", prompt: testEditPrompt, hasImage: true, n: intPtr(16), wantErr: ErrImageCountRange},
		{name: "refs plus n too large", prompt: testEditPrompt, imagesLen: 14, n: intPtr(2), wantErr: ErrImageRefPlusCount},
		{name: "refs plus n at limit", prompt: testEditPrompt, imagesLen: 14, n: intPtr(1), wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateImageEdit(tt.prompt, tt.hasImage, tt.imagesLen, tt.n)
			require.ErrorIs(t, err, tt.wantErr, "ValidateImageEdit = %v, want %v", err, tt.wantErr)
		})
	}
}

func TestValidateVideoGeneration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		prompt   string
		hasImage bool
		refCount int
		wantErr  error
	}{
		{name: "prompt only", prompt: testSunsetPrompt, wantErr: nil},
		{name: "image only", hasImage: true, wantErr: nil},
		{name: "refs with prompt", prompt: testSunsetPrompt, refCount: 3, wantErr: nil},
		{name: "too many refs", prompt: testSunsetPrompt, refCount: 8, wantErr: ErrTooManyVideoRefs},
		{name: "nothing provided", wantErr: ErrVideoInputRequired},
		{name: "refs without prompt", refCount: 2, wantErr: ErrVideoRefPromptRequired},
		{name: "mixed refs and image", prompt: testSunsetPrompt, hasImage: true, refCount: 1, wantErr: ErrVideoRefMixed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateVideoGeneration(tt.prompt, tt.hasImage, tt.refCount)
			require.ErrorIs(t, err, tt.wantErr, "ValidateVideoGeneration = %v, want %v", err, tt.wantErr)
		})
	}
}

func TestVideoDurationLimitsAndValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		model    string
		wantMin  int
		wantMax  int
		duration int
		wantErr  bool
	}{
		{name: "pro model", model: proVideoModel, wantMin: 4, wantMax: 15, duration: 4},
		{name: "pro model too short", model: proVideoModel, wantMin: 4, wantMax: 15, duration: 3, wantErr: true},
		{name: "fast model", model: fastVideoModel, wantMin: 2, wantMax: 12, duration: 12},
		{name: "fast model too long", model: fastVideoModel, wantMin: 2, wantMax: 12, duration: 13, wantErr: true},
		{name: "unknown model", model: "other", wantMin: 1, wantMax: 15, duration: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotMin, gotMax := VideoDurationLimits(tt.model)
			require.Equal(t, tt.wantMin, gotMin, "VideoDurationLimits(%q) = (%d, %d), want (%d, %d)", tt.model, gotMin, gotMax, tt.wantMin, tt.wantMax)
			require.Equal(t, tt.wantMax, gotMax, "VideoDurationLimits(%q) = (%d, %d), want (%d, %d)", tt.model, gotMin, gotMax, tt.wantMin, tt.wantMax)

			err := ValidateVideoDuration(tt.model, tt.duration)
			require.Equal(t, tt.wantErr, (err != nil), "ValidateVideoDuration(%q, %d) = %v, wantErr %v", tt.model, tt.duration, err, tt.wantErr)
		})
	}
}

func TestResolveDuration(t *testing.T) {
	t.Parallel()

	{
		got, ok := ResolveDuration(intPtr(5), intPtr(8))
		require.Equal(t, 5, got, "ResolveDuration prefers duration, got (%d, %v)", got, ok)
		require.True(t, ok, "ResolveDuration prefers duration, got (%d, %v)", got, ok)
	}

	{
		got, ok := ResolveDuration(nil, intPtr(8))
		require.Equal(t, 8, got, "ResolveDuration falls back to seconds, got (%d, %v)", got, ok)
		require.True(t, ok, "ResolveDuration falls back to seconds, got (%d, %v)", got, ok)
	}

	{
		got, ok := ResolveDuration(nil, nil)
		require.Equal(t, 0, got, "ResolveDuration with none = (%d, %v)", got, ok)
		require.False(t, ok, "ResolveDuration with none = (%d, %v)", got, ok)
	}
}

func TestNormalizeVideoResolutionAndRatio(t *testing.T) {
	t.Parallel()

	{
		got := NormalizeVideoResolution(" 1080P ")
		require.Equal(t, "1080p", got, "NormalizeVideoResolution = %q", got)
	}

	{
		got := NormalizeVideoResolution("8k")
		require.Empty(t, got, "NormalizeVideoResolution(8k) = %q, want empty", got)
	}

	{
		got := NormalizeVideoRatio(" 16:9 ")
		require.Equal(t, test169, got, "NormalizeVideoRatio = %q", got)
	}

	{
		got := NormalizeVideoRatio("5:4")
		require.Empty(t, got, "NormalizeVideoRatio(5:4) = %q, want empty", got)
	}
}

func TestIsAllowedAssetHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		host string
		want bool
	}{
		{host: "ark-content.volces.com", want: true},
		{host: "VOLCES.COM", want: true},
		{host: "p9-aiop-sign.byteimg.com", want: true},
		{host: "tos-cn-beijing.volces.com", want: true},
		{host: "evil.com", want: false},
		{host: "notvolces.com.attacker.io", want: false},
		{host: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			t.Parallel()

			{
				got := IsAllowedAssetHost(tt.host)
				require.Equal(t, tt.want, got, "IsAllowedAssetHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}
