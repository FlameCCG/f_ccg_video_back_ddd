package ai

import (
	"strings"

	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/config"
	aierrors "f_ccg_video_back_ddd/internal/domain/ai/domainerr"
	aivo "f_ccg_video_back_ddd/internal/domain/ai/valueobject"

	"go.uber.org/zap"
)

// Service 编排 AI 对话与媒体生成用例。
//
// 配置通过 Provider 读取，上游能力通过端口注入；协议细节由 infrastructure/external/ark 实现。
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

// snapshot 在公开用例入口捕获一份 AI 配置，后续链路不再读 Provider。
func (s *Service) snapshot() (configSnapshot, error) {
	root := s.cfgProvider.Get()
	if root == nil {
		return configSnapshot{}, aierrors.ErrConfigMissing
	}

	source := root.AI
	if source == nil {
		return configSnapshot{}, aierrors.ErrConfigMissing
	}

	cfg := *source
	cfg.ChatModels = append([]config.AIModel{}, source.ChatModels...)
	cfg.ThinkingEfforts = append([]config.AIModel{}, source.ThinkingEfforts...)
	cfg.ImageModels = append([]config.AIModel{}, source.ImageModels...)
	cfg.VideoModels = append([]config.AIModel{}, source.VideoModels...)
	cfg.ChatModelBaseURL = aivo.NormalizeBaseURL(cfg.ChatModelBaseURL)

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
		return aierrors.ErrChatAPIKeyMissing
	}

	return nil
}

func requireDoubaoAPIKey(cfg *config.AI) error {
	if cfg == nil || strings.TrimSpace(cfg.DoubaoModelAPIKey) == "" {
		return aierrors.ErrDoubaoAPIKeyMissing
	}

	return nil
}
