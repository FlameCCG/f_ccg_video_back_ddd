package ai

import (
	"strings"

	"f_ccg_video_back_ddd/internal/config"
	aidomain "f_ccg_video_back_ddd/internal/domain/ai"
)

// PublicOptions 返回前台可用的模型与思考配置（脱敏）.
func (s *Service) PublicOptions() (PublicOptions, error) {
	snapshot, err := s.snapshot()
	if err != nil {
		return PublicOptions{}, err
	}
	cfg := &snapshot.settings

	return PublicOptions{
		ChatModel:       strings.TrimSpace(cfg.ChatModel),
		ChatModels:      aidomain.Options(toOptions(cfg.ChatModels)).Clone(),
		ThinkingEnabled: cfg.ThinkingEnabled,
		ThinkingEffort:  (aidomain.PromptVideoCandidate{}).FirstNonEmpty(strings.TrimSpace(cfg.ThinkingEffort), "high"),
		ThinkingEfforts: aidomain.Options(toOptions(cfg.ThinkingEfforts)).Clone(),
		ImageModel:      strings.TrimSpace(cfg.ImageModel),
		ImageModels:     aidomain.Options(toOptions(cfg.ImageModels)).Clone(),
		VideoModel:      strings.TrimSpace(cfg.VideoModel),
		VideoModels:     aidomain.Options(toOptions(cfg.VideoModels)).Clone(),
	}, nil
}

// toOptions 配置选项列表 → 领域选项列表。
func toOptions(src []config.AIModel) []aidomain.Option {
	if len(src) == 0 {
		return nil
	}

	out := make([]aidomain.Option, 0, len(src))
	for _, item := range src {
		out = append(out, aidomain.Option{Label: item.Label, Value: item.Value})
	}

	return out
}
