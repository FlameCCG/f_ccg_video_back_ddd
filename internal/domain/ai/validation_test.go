package ai

import (
	"errors"
	"testing"
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
		{name: "ok without n", prompt: "一只猫", wantErr: nil},
		{name: "ok with n", prompt: "一只猫", n: intPtr(4), wantErr: nil},
		{name: "empty prompt", prompt: "   ", wantErr: ErrEmptyPrompt},
		{name: "zero n", prompt: "一只猫", n: intPtr(0), wantErr: ErrImageCountRange},
		{name: "n too large", prompt: "一只猫", n: intPtr(16), wantErr: ErrImageCountRange},
		{name: "n at max", prompt: "一只猫", n: intPtr(15), wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := ValidateImageGeneration(tt.prompt, tt.n); !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateImageGeneration = %v, want %v", err, tt.wantErr)
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
		{name: "single image ok", prompt: "换背景", hasImage: true, wantErr: nil},
		{name: "images ok", prompt: "换背景", imagesLen: 3, n: intPtr(2), wantErr: nil},
		{name: "empty prompt", prompt: "", hasImage: true, wantErr: ErrEmptyPrompt},
		{name: "neither provided", prompt: "换背景", wantErr: ErrImageRefExclusive},
		{name: "both provided", prompt: "换背景", hasImage: true, imagesLen: 2, wantErr: ErrImageRefExclusive},
		{name: "too many refs", prompt: "换背景", imagesLen: 15, wantErr: ErrTooManyImageRefs},
		{name: "n out of range", prompt: "换背景", hasImage: true, n: intPtr(16), wantErr: ErrImageCountRange},
		{name: "refs plus n too large", prompt: "换背景", imagesLen: 14, n: intPtr(2), wantErr: ErrImageRefPlusCount},
		{name: "refs plus n at limit", prompt: "换背景", imagesLen: 14, n: intPtr(1), wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateImageEdit(tt.prompt, tt.hasImage, tt.imagesLen, tt.n)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateImageEdit = %v, want %v", err, tt.wantErr)
			}
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
		{name: "prompt only", prompt: "海边日落", wantErr: nil},
		{name: "image only", hasImage: true, wantErr: nil},
		{name: "refs with prompt", prompt: "海边日落", refCount: 3, wantErr: nil},
		{name: "too many refs", prompt: "海边日落", refCount: 8, wantErr: ErrTooManyVideoRefs},
		{name: "nothing provided", wantErr: ErrVideoInputRequired},
		{name: "refs without prompt", refCount: 2, wantErr: ErrVideoRefPromptRequired},
		{name: "mixed refs and image", prompt: "海边日落", hasImage: true, refCount: 1, wantErr: ErrVideoRefMixed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateVideoGeneration(tt.prompt, tt.hasImage, tt.refCount)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateVideoGeneration = %v, want %v", err, tt.wantErr)
			}
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
			if gotMin != tt.wantMin || gotMax != tt.wantMax {
				t.Fatalf("VideoDurationLimits(%q) = (%d, %d), want (%d, %d)",
					tt.model, gotMin, gotMax, tt.wantMin, tt.wantMax)
			}

			err := ValidateVideoDuration(tt.model, tt.duration)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateVideoDuration(%q, %d) = %v, wantErr %v", tt.model, tt.duration, err, tt.wantErr)
			}
		})
	}
}

func TestResolveDuration(t *testing.T) {
	t.Parallel()

	if got, ok := ResolveDuration(intPtr(5), intPtr(8)); got != 5 || !ok {
		t.Fatalf("ResolveDuration prefers duration, got (%d, %v)", got, ok)
	}

	if got, ok := ResolveDuration(nil, intPtr(8)); got != 8 || !ok {
		t.Fatalf("ResolveDuration falls back to seconds, got (%d, %v)", got, ok)
	}

	if got, ok := ResolveDuration(nil, nil); got != 0 || ok {
		t.Fatalf("ResolveDuration with none = (%d, %v)", got, ok)
	}
}

func TestNormalizeVideoResolutionAndRatio(t *testing.T) {
	t.Parallel()

	if got := NormalizeVideoResolution(" 1080P "); got != "1080p" {
		t.Fatalf("NormalizeVideoResolution = %q", got)
	}

	if got := NormalizeVideoResolution("8k"); got != "" {
		t.Fatalf("NormalizeVideoResolution(8k) = %q, want empty", got)
	}

	if got := NormalizeVideoRatio(" 16:9 "); got != "16:9" {
		t.Fatalf("NormalizeVideoRatio = %q", got)
	}

	if got := NormalizeVideoRatio("5:4"); got != "" {
		t.Fatalf("NormalizeVideoRatio(5:4) = %q, want empty", got)
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

			if got := IsAllowedAssetHost(tt.host); got != tt.want {
				t.Fatalf("IsAllowedAssetHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}
