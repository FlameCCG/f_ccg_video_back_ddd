package ai

import (
	"context"
	"strings"

	"f_ccg_video_back_ddd/internal/config"
	aidomain "f_ccg_video_back_ddd/internal/domain/ai"
)

// GenerateImage 调用上游文生图接口.
func (s *Service) GenerateImage(ctx context.Context, req ImageGenerationRequest) (any, error) {
	snapshot, err := s.snapshot()
	if err != nil {
		return nil, err
	}

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
func (s *Service) ensureImageResultCount(
	ctx context.Context,
	runtime RuntimeConfig,
	req ImageGenerationRequest,
	refs []string,
	result any,
) (any, error) {
	want := (aidomain.GenerationPolicy{}).RequestedImageCount(req.N)
	if want <= 1 {
		return result, nil
	}

	current := imageGenerationPayload{raw: result}
	for attempts := 0; current.count() < want && attempts < want; attempts++ {
		singleReq := req
		one := 1
		singleReq.N = &one

		next, err := s.images.Generate(ctx, toImageInvocation(runtime, singleReq, refs))
		if err != nil {
			return nil, err
		}

		merged, grew := current.merge(imageGenerationPayload{raw: next}, want)
		if !grew {
			break
		}

		result = merged
		current = imageGenerationPayload{raw: result}
	}

	return result, nil
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
