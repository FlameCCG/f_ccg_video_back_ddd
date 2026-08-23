package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/config"
	domain "f_ccg_video_back_ddd/internal/domain/ai"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// defaultMaxOutputTokens 聊天正式回答的最大输出 token 数。
const defaultMaxOutputTokens = 4096

// Service AI 用例服务（原 service/ai_service.Service + 包级 GetPublicOptions）。
//
// 原实现直读 global.Config() 并在包内直接发起上游 HTTP / eino 调用；
// 新架构改为注入配置 Provider 与四个上游端口，上游协议细节下沉到
// infrastructure/external/ark。
type Service struct {
	cfgProvider shared.ConfigReader
	text        TextGenerator
	images      ImageGenerator
	videos      VideoTaskClient
	assets      AssetFetcher
	prompts     PromptVideoQuery
	vectors     VideoVectorSearch
	logger      *zap.Logger
}

type configSnapshot struct {
	settings config.AI
	runtime  RuntimeConfig
}

// NewService 构造 AI 用例服务。
func NewService(
	cfgProvider shared.ConfigReader,
	text TextGenerator,
	images ImageGenerator,
	videos VideoTaskClient,
	assets AssetFetcher,
	prompts PromptVideoQuery,
	vectors VideoVectorSearch,
	logger *zap.Logger,
) *Service {
	return &Service{
		cfgProvider: cfgProvider,
		text:        text,
		images:      images,
		videos:      videos,
		assets:      assets,
		prompts:     prompts,
		vectors:     vectors,
		logger:      logger,
	}
}

// CreateResponse 调用聊天模型生成非流式文本响应.
func (s *Service) CreateResponse(ctx context.Context, req ResponsesRequest) (any, error) {
	snapshot, err := s.snapshot()
	if err != nil {
		return nil, err
	}

	prepared, conv, err := s.prepareResponsesRequest(ctx, &snapshot.settings, snapshot.runtime, req)
	if err != nil {
		return nil, err
	}

	text, err := s.text.InvokeText(ctx, TextInvocation{
		Runtime:        snapshot.runtime,
		Model:          prepared.Model,
		Messages:       conv.modelMessages(),
		Thinking:       prepared.Thinking,
		ThinkingEffort: prepared.ThinkingEffort,
		MaxTokens:      defaultMaxOutputTokens,
	})
	if err != nil {
		return nil, err
	}

	return buildResponsesCompatiblePayload(prepared.Model, text), nil
}

// OpenResponseStream 打开聊天模型流式响应.
func (s *Service) OpenResponseStream(
	ctx context.Context,
	req ResponsesRequest,
) (*schema.StreamReader[*schema.Message], error) {
	snapshot, err := s.snapshot()
	if err != nil {
		return nil, err
	}

	prepared, conv, err := s.prepareResponsesRequest(ctx, &snapshot.settings, snapshot.runtime, req)
	if err != nil {
		return nil, err
	}

	return s.text.OpenStream(ctx, TextInvocation{
		Runtime:        snapshot.runtime,
		Model:          prepared.Model,
		Messages:       conv.modelMessages(),
		Thinking:       prepared.Thinking,
		ThinkingEffort: prepared.ThinkingEffort,
		MaxTokens:      defaultMaxOutputTokens,
	})
}

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
		return nil, domain.ErrEmptyRequestID
	}

	if err = requireDoubaoAPIKey(&snapshot.settings); err != nil {
		return nil, err
	}

	return s.videos.Get(ctx, snapshot.runtime, requestID)
}

// OpenAssetStream 代理拉取上游资源流（仅放行白名单域名，防 SSRF）.
func (s *Service) OpenAssetStream(
	ctx context.Context,
	assetURL string,
	incomingHeaders http.Header,
) (*http.Response, error) {
	snapshot, err := s.snapshot()
	if err != nil {
		return nil, err
	}

	parsed, err := url.Parse(strings.TrimSpace(assetURL))
	if err != nil {
		return nil, fmt.Errorf("解析资源地址失败: %w", err)
	}

	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, domain.ErrUnsupportedAssetScheme
	}

	if !domain.IsAllowedAssetHost(parsed.Hostname()) {
		return nil, domain.ErrAssetHostNotAllowed
	}

	return s.assets.Open(ctx, snapshot.runtime, parsed.String(), incomingHeaders)
}

// PublicOptions 返回前台可用的模型与思考配置（脱敏）.
func (s *Service) PublicOptions() (PublicOptions, error) {
	snapshot, err := s.snapshot()
	if err != nil {
		return PublicOptions{}, err
	}
	cfg := &snapshot.settings

	return PublicOptions{
		ChatModel:       strings.TrimSpace(cfg.ChatModel),
		ChatModels:      domain.CloneOptions(toOptions(cfg.ChatModels)),
		ThinkingEnabled: cfg.ThinkingEnabled,
		ThinkingEffort:  domain.FirstNonEmpty(strings.TrimSpace(cfg.ThinkingEffort), "high"),
		ThinkingEfforts: domain.CloneOptions(toOptions(cfg.ThinkingEfforts)),
		ImageModel:      strings.TrimSpace(cfg.ImageModel),
		ImageModels:     domain.CloneOptions(toOptions(cfg.ImageModels)),
		VideoModel:      strings.TrimSpace(cfg.VideoModel),
		VideoModels:     domain.CloneOptions(toOptions(cfg.VideoModels)),
	}, nil
}

// prepareResponsesRequest 校验并补全聊天请求：模型白名单、思考选项与系统提示词注入。
func (s *Service) prepareResponsesRequest(
	ctx context.Context,
	cfg *config.AI,
	runtime RuntimeConfig,
	req ResponsesRequest,
) (ResponsesRequest, conversation, error) {
	var conv conversation

	if err := requireChatAPIKey(cfg); err != nil {
		return req, conv, err
	}

	if !domain.HasRawJSON(req.Input) {
		return req, conv, domain.ErrEmptyInput
	}

	var err error
	req.Model, err = domain.NormalizeConfiguredModel(req.Model, cfg.ChatModel, toOptions(cfg.ChatModels), "聊天")
	if err != nil {
		return req, conv, err
	}

	req, err = normalizeThinkingOptions(cfg, req)
	if err != nil {
		return req, conv, err
	}

	conv, err = parseConversation(req.Input)
	if err != nil {
		return req, conv, err
	}
	conv = conv.trimHistory(cfg)
	if !conv.HasMessages() {
		return req, conv, domain.ErrNoUserMessage
	}

	prompt, promptErr := s.buildResponsesSystemPrompt(ctx, cfg, runtime, conv, req.Model)
	if promptErr != nil {
		s.logger.Warn("build AI responses system prompt failed", zap.Error(promptErr))
	}

	conv = conv.prependSystem(domain.FirstNonEmpty(prompt, cfg.SystemPrompt))

	return req, conv, nil
}

func prepareImageGenerationRequest(
	cfg *config.AI,
	req ImageGenerationRequest,
) (ImageGenerationRequest, error) {
	if err := requireDoubaoAPIKey(cfg); err != nil {
		return req, err
	}

	if err := domain.ValidateImageGeneration(req.Prompt, req.N); err != nil {
		return req, err
	}

	var err error
	req.Model, err = domain.NormalizeConfiguredModel(req.Model, cfg.ImageModel, toOptions(cfg.ImageModels), "图片")
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
	if err := domain.ValidateImageEdit(req.Prompt, hasImage, len(req.Images), req.N); err != nil {
		return req, err
	}

	var err error
	req.Model, err = domain.NormalizeConfiguredModel(req.Model, cfg.ImageModel, toOptions(cfg.ImageModels), "图片")
	if err != nil {
		return req, err
	}

	return req, nil
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

	if err := domain.ValidateVideoGeneration(req.Prompt, hasImage, len(req.ReferenceImages)); err != nil {
		return req, err
	}

	var err error
	req.Model, err = domain.NormalizeConfiguredModel(req.Model, cfg.VideoModel, toOptions(cfg.VideoModels), "视频")
	if err != nil {
		return req, err
	}

	if duration, ok := domain.ResolveDuration(req.Duration, req.Seconds); ok {
		if err = domain.ValidateVideoDuration(req.Model, duration); err != nil {
			return req, err
		}
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
	want := domain.RequestedImageCount(req.N)
	if want <= 1 {
		return result, nil
	}

	for attempts := 0; domain.ImageResultCount(result) < want && attempts < want; attempts++ {
		singleReq := req
		one := 1
		singleReq.N = &one

		next, err := s.images.Generate(ctx, toImageInvocation(runtime, singleReq, refs))
		if err != nil {
			return nil, err
		}

		merged, grew := domain.MergeImageGenerationResponses(result, next, want)
		if !grew {
			break
		}

		result = merged
	}

	return result, nil
}

// snapshot 在公开用例入口捕获一份 AI 配置，后续链路不再读 Provider。
func (s *Service) snapshot() (configSnapshot, error) {
	root := s.cfgProvider.Get()
	if root == nil {
		return configSnapshot{}, domain.ErrConfigMissing
	}

	source := root.AI
	if source == nil {
		return configSnapshot{}, domain.ErrConfigMissing
	}

	cfg := *source
	cfg.ChatModels = append([]config.AIModel{}, source.ChatModels...)
	cfg.ThinkingEfforts = append([]config.AIModel{}, source.ThinkingEfforts...)
	cfg.ImageModels = append([]config.AIModel{}, source.ImageModels...)
	cfg.VideoModels = append([]config.AIModel{}, source.VideoModels...)
	cfg.ChatModelBaseURL = domain.NormalizeBaseURL(cfg.ChatModelBaseURL)

	return configSnapshot{
		settings: cfg,
		runtime: RuntimeConfig{
			ChatModelBaseURL: cfg.ChatModelBaseURL,
			ChatModelAPIKey:  strings.TrimSpace(cfg.ChatModelAPIKey),
			DoubaoAPIKey:     strings.TrimSpace(cfg.DoubaoModelAPIKey),
			TimeoutSec:       cfg.TimeoutSec,
		},
	}, nil
}

func requireChatAPIKey(cfg *config.AI) error {
	if cfg == nil || strings.TrimSpace(cfg.ChatModelAPIKey) == "" {
		return domain.ErrChatAPIKeyMissing
	}

	return nil
}

func requireDoubaoAPIKey(cfg *config.AI) error {
	if cfg == nil || strings.TrimSpace(cfg.DoubaoModelAPIKey) == "" {
		return domain.ErrDoubaoAPIKeyMissing
	}

	return nil
}

// normalizeThinkingOptions 按后台开关规范化本次请求的思考选项。
func normalizeThinkingOptions(cfg *config.AI, req ResponsesRequest) (ResponsesRequest, error) {
	if cfg == nil || !cfg.ThinkingEnabled {
		disabled := false
		req.Thinking = &disabled
		req.ThinkingEffort = ""

		return req, nil
	}

	// 未传 thinking 时默认开启（与后台开关一致）
	if req.Thinking == nil {
		enabled := true
		req.Thinking = &enabled
	}

	if req.Thinking != nil && !*req.Thinking {
		req.ThinkingEffort = ""

		return req, nil
	}

	effort, err := domain.NormalizeConfiguredOption(
		req.ThinkingEffort,
		cfg.ThinkingEffort,
		toOptions(cfg.ThinkingEfforts),
		"思考强度",
		"high",
	)
	if err != nil {
		return req, err
	}

	req.ThinkingEffort = effort

	return req, nil
}

// buildResponsesCompatiblePayload 组装 Responses API 兼容的非流式响应载荷。
func buildResponsesCompatiblePayload(model, text string) map[string]any {
	return map[string]any{
		"status": "completed",
		"model":  model,
		"output": []map[string]any{
			{
				"type": "message",
				"content": []map[string]any{
					{
						"type": "output_text",
						"text": text,
					},
				},
			},
		},
	}
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

// toOptions 配置选项列表 → 领域选项列表。
func toOptions(src []config.AIModel) []domain.Option {
	if len(src) == 0 {
		return nil
	}

	out := make([]domain.Option, 0, len(src))
	for _, item := range src {
		out = append(out, domain.Option{Label: item.Label, Value: item.Value})
	}

	return out
}
