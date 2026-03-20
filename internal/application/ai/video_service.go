package ai

import (
	"context"
	"strings"

	"f_ccg_video_back_ddd/internal/config"
	aierrors "f_ccg_video_back_ddd/internal/domain/ai/domainerr"
	aivo "f_ccg_video_back_ddd/internal/domain/ai/valueobject"
)

// GenerateVideo 创建上游视频生成任务.
func (s *Service) GenerateVideo(ctx context.Context, req VideoGenerationRequest) (any, error) {
	snapshot, err := s.snapshot()
	if err != nil {
		return nil, err
	}

	prepared, err := prepareVideoGenerationRequest(&snapshot.settings, req)
	if err != nil {
		return nil, err
	}

	return s.videos.Create(ctx, toVideoInvocation(snapshot.runtime, prepared))
}

// GetVideo 查询视频生成任务状态.
func (s *Service) GetVideo(ctx context.Context, requestID string) (any, error) {
	snapshot, err := s.snapshot()
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(requestID) == "" {
		return nil, aierrors.ErrEmptyRequestID
	}

	if err = requireDoubaoAPIKey(&snapshot.settings); err != nil {
		return nil, err
	}

	return s.videos.Get(ctx, snapshot.runtime, requestID)
}

func prepareVideoGenerationRequest(
	cfg *config.AI,
	req VideoGenerationRequest,
) (VideoGenerationRequest, error) {
	if err := requireDoubaoAPIKey(cfg); err != nil {
		return req, err
	}

	hasImage := (req.Image != nil && !req.Image.IsZero()) ||
		(req.InputReference != nil && !req.InputReference.IsZero())

	if err := aivo.ValidateVideoGeneration(req.Prompt, hasImage, len(req.ReferenceImages)); err != nil {
		return req, err
	}

	var err error
	req.Model, err = aivo.NormalizeConfiguredModel(req.Model, cfg.VideoModel, toOptions(cfg.VideoModels), "视频")
	if err != nil {
		return req, err
	}

	if duration, ok := aivo.ResolveDuration(req.Duration, req.Seconds); ok {
		if err = aivo.ValidateVideoDuration(req.Model, duration); err != nil {
			return req, err
		}
	}

	return req, nil
}

// videoImageRefs 按「首帧图 → input_reference → 参考图组」优先级提取视频参考图。
func videoImageRefs(req VideoGenerationRequest) []string {
	if req.Image != nil && !req.Image.IsZero() {
		return []string{strings.TrimSpace(req.Image.URL)}
	}

	if req.InputReference != nil && !req.InputReference.IsZero() {
		return []string{strings.TrimSpace(req.InputReference.URL)}
	}

	out := make([]string, 0, len(req.ReferenceImages))

	for _, image := range req.ReferenceImages {
		if !image.IsZero() {
			out = append(out, strings.TrimSpace(image.URL))
		}
	}

	return out
}

func toVideoInvocation(runtime RuntimeConfig, req VideoGenerationRequest) VideoInvocation {
	return VideoInvocation{
		Runtime:       runtime,
		Model:         req.Model,
		Prompt:        req.Prompt,
		Duration:      req.Duration,
		Seconds:       req.Seconds,
		AspectRatio:   req.AspectRatio,
		Resolution:    req.Resolution,
		Size:          req.Size,
		ImageRefs:     videoImageRefs(req),
		GenerateAudio: req.GenerateAudio,
		CameraFixed:   req.CameraFixed,
		Watermark:     req.Watermark,
		Seed:          req.Seed,
	}
}
