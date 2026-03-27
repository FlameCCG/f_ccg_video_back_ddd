package ai

import (
	"context"
	"strings"
	"unicode/utf8"

	"f_ccg_video_back_ddd/internal/config"
	aidomain "f_ccg_video_back_ddd/internal/domain/ai"

	"go.uber.org/zap"
)

// GenerateImage 调用上游文生图接口.
func (s *Service) GenerateImage(ctx context.Context, req ImageGenerationRequest) (any, error) {
	snapshot, err := s.snapshot()
	if err != nil {
		return nil, err
	}

	ctx, cancel := snapshot.runtime.WithContext(ctx)
	defer cancel()
	prepared, err := prepareImageGenerationRequest(&snapshot.settings, req)
	if err != nil {
		return nil, err
	}

	result, err := s.images.Generate(ctx, toImageInvocation(snapshot.runtime, prepared, nil))
	if err != nil {
		return nil, err
	}

	return s.ensureImageResultCount(ctx, snapshot.runtime, prepared, nil, result)
}

// EditImage 调用上游图生图/图片编辑接口.
func (s *Service) EditImage(ctx context.Context, req ImageEditRequest) (any, error) {
	snapshot, err := s.snapshot()
	if err != nil {
		return nil, err
	}

	ctx, cancel := snapshot.runtime.WithContext(ctx)
	defer cancel()
	prepared, err := prepareImageEditRequest(&snapshot.settings, req)
	if err != nil {
		return nil, err
	}

	imageReq := ImageGenerationRequest{
		Model:          prepared.Model,
		Prompt:         prepared.Prompt,
		N:              prepared.N,
		AspectRatio:    prepared.AspectRatio,
		Resolution:     prepared.Resolution,
		ResponseFormat: prepared.ResponseFormat,
		Size:           prepared.Size,
	}
	refs := imageRefsForEdit(prepared)

	result, err := s.images.Generate(ctx, toImageInvocation(snapshot.runtime, imageReq, refs))
	if err != nil {
		return nil, err
	}

	return s.ensureImageResultCount(ctx, snapshot.runtime, imageReq, refs, result)
}

func prepareImageGenerationRequest(
	cfg *config.AI,
	req ImageGenerationRequest,
) (ImageGenerationRequest, error) {
	if err := (aidomain.InputPolicy{}).ValidateMessage(utf8.RuneCountInString(req.Prompt), cfg.InputMaxChars); err != nil {
		return req, err
	}
	if err := requireDoubaoAPIKey(cfg); err != nil {
		return req, err
	}

	policy := aidomain.GenerationPolicy{}
	if err := policy.ValidateImageGeneration(req.Prompt, req.N); err != nil {
		return req, err
	}

	var err error
	req.Model, err = aidomain.Options(toOptions(cfg.ImageModels)).NormalizeConfiguredModel(req.Model, cfg.ImageModel, "图片")
	if err != nil {
		return req, err
	}

	return req, nil
}

func prepareImageEditRequest(cfg *config.AI, req ImageEditRequest) (ImageEditRequest, error) {
	if err := (aidomain.InputPolicy{}).ValidateMessage(utf8.RuneCountInString(req.Prompt), cfg.InputMaxChars); err != nil {
		return req, err
	}
	if err := requireDoubaoAPIKey(cfg); err != nil {
		return req, err
	}

	hasImage := req.Image != nil && !req.Image.IsZero()
	if err := (aidomain.GenerationPolicy{}).ValidateImageEdit(req.Prompt, hasImage, len(req.Images), req.N); err != nil {
		return req, err
	}

	var err error
	req.Model, err = aidomain.Options(toOptions(cfg.ImageModels)).NormalizeConfiguredModel(req.Model, cfg.ImageModel, "图片")
	if err != nil {
		return req, err
	}

	return req, nil
}

// ensureImageResultCount 上游一次未返回足够张数时逐张补图，最多补 want 次。
func (s *Service) ensureImageResultCount(ctx context.Context, runtime RuntimeConfig, req ImageGenerationRequest, refs []string, result any) (any, error) {
	want := (aidomain.GenerationPolicy{}).RequestedImageCount(req.N)
	current := imageGenerationPayload{raw: result}
	if want == 1 && current.count() == 0 {
		return nil, aidomain.ErrImageOutputEmpty
	}
	attempts := max(want-current.count(), 0)
	for range attempts {
		if current.count() >= want {
			break
		}
		singleReq := req
		one := 1
		singleReq.N = &one
		next, err := s.images.Generate(ctx, toImageInvocation(runtime, singleReq, refs))
		if err != nil {
			if current.count() == 0 {
				return nil, err
			}
			s.log().Warn("AI image supplement failed; returning available images", zap.Int("requested", want), zap.Int("returned", current.count()), zap.Error(err))
			break
		}
		merged, grew := current.merge(imageGenerationPayload{raw: next}, want)
		if !grew {
			break
		}
		current = imageGenerationPayload{raw: merged}
	}
	if current.count() == 0 {
		return nil, aidomain.ErrImageOutputEmpty
	}
	if current.count() < want {
		return current.partialResult(want), nil
	}
	return current.raw, nil
}

// imageRefsForEdit 提取图片编辑请求中的参考图地址。
func imageRefsForEdit(req ImageEditRequest) []string {
	if req.Image != nil && !req.Image.IsZero() {
		return []string{strings.TrimSpace(req.Image.URL)}
	}

	refs := make([]string, 0, len(req.Images))

	for _, image := range req.Images {
		if imageURL := strings.TrimSpace(image.URL); imageURL != "" {
			refs = append(refs, imageURL)
		}
	}

	return refs
}

func toImageInvocation(runtime RuntimeConfig, req ImageGenerationRequest, refs []string) ImageInvocation {
	return ImageInvocation{
		Runtime:        runtime,
		Model:          req.Model,
		Prompt:         req.Prompt,
		N:              req.N,
		AspectRatio:    req.AspectRatio,
		Resolution:     req.Resolution,
		ResponseFormat: req.ResponseFormat,
		Size:           req.Size,
		Refs:           refs,
	}
}
