package ai

import (
	"context"
	"errors"
	"strings"
	"testing"

	aidomain "f_ccg_video_back_ddd/internal/domain/ai"
	"go.uber.org/zap"
)

type supplementImageGenerator struct {
	calls int
	err   error
}

func (g *supplementImageGenerator) Generate(context.Context, ImageInvocation) (any, error) {
	g.calls++
	return nil, g.err
}

func TestImageSupplementPreservesSuccessfulResultsOnError(t *testing.T) {
	t.Parallel()
	upstream := &supplementImageGenerator{err: errors.New("private upstream credentials")}
	s := &Service{images: upstream, logger: zap.NewNop()}
	want := 3
	initial := map[string]any{"model": "image-model", "data": []any{map[string]any{"url": "https://example.com/generated.png"}}}
	result, err := s.ensureImageResultCount(t.Context(), RuntimeConfig{}, ImageGenerationRequest{N: &want}, nil, initial)
	if err != nil {
		t.Fatal(err)
	}
	partial := result.(map[string]any)
	if partial["partial"] != true || partial["returnedCount"] != 1 || partial["requestedCount"] != 3 || (imageGenerationPayload{raw: result}).count() != 1 || upstream.calls != 1 {
		t.Fatalf("partial result=%+v calls=%d", partial, upstream.calls)
	}
	if strings.Contains(partial["message"].(string), "private") {
		t.Fatal("upstream details leaked")
	}
	if _, changed := initial["partial"]; changed {
		t.Fatal("original response mutated")
	}
}

func TestEmptySingleImageResponseFailsWithoutRegenerating(t *testing.T) {
	t.Parallel()
	upstream := &supplementImageGenerator{}
	s := &Service{images: upstream}
	result, err := s.ensureImageResultCount(t.Context(), RuntimeConfig{}, ImageGenerationRequest{}, nil, map[string]any{"data": []any{}})
	if !errors.Is(err, aidomain.ErrImageOutputEmpty) || result != nil || upstream.calls != 0 {
		t.Fatalf("result=%v error=%v extraCalls=%d", result, err, upstream.calls)
	}
}

func TestVideoImageRefsPreserveGenerationMode(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		request VideoGenerationRequest
		role    string
	}{
		{name: "first frame", request: VideoGenerationRequest{Image: &ImageRef{URL: " https://example.com/image.png "}}, role: VideoImageFirstFrame},
		{name: "input reference alias", request: VideoGenerationRequest{InputReference: &ImageRef{URL: "https://example.com/image.png"}}, role: VideoImageFirstFrame},
		{name: "reference", request: VideoGenerationRequest{ReferenceImages: []ImageRef{{URL: "https://example.com/image.png"}}}, role: VideoImageReference},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			inv := toVideoInvocation(RuntimeConfig{}, tc.request)
			if len(inv.ImageRefs) != 1 || inv.ImageRefs[0].Role != tc.role || inv.ImageRefs[0].URL != "https://example.com/image.png" {
				t.Fatalf("image refs=%+v", inv.ImageRefs)
			}
		})
	}
}
