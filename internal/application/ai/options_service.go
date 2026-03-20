package ai

import (
	"strings"

	"f_ccg_video_back_ddd/internal/config"
	aivo "f_ccg_video_back_ddd/internal/domain/ai/valueobject"
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
		ChatModels:      aivo.CloneOptions(toOptions(cfg.ChatModels)),
		ThinkingEnabled: cfg.ThinkingEnabled,
		ThinkingEffort:  aivo.FirstNonEmpty(strings.TrimSpace(cfg.ThinkingEffort), "high"),
		ThinkingEfforts: aivo.CloneOptions(toOptions(cfg.ThinkingEfforts)),
		ImageModel:      strings.TrimSpace(cfg.ImageModel),
		ImageModels:     aivo.CloneOptions(toOptions(cfg.ImageModels)),
		VideoModel:      strings.TrimSpace(cfg.VideoModel),
		VideoModels:     aivo.CloneOptions(toOptions(cfg.VideoModels)),
	}, nil
}

// toOptions 配置选项列表 → 领域选项列表。
func toOptions(src []config.AIModel) []aivo.Option {
	if len(src) == 0 {
		return nil
	}

	out := make([]aivo.Option, 0, len(src))
	for _, item := range src {
		out = append(out, aivo.Option{Label: item.Label, Value: item.Value})
	}

	return out
}
