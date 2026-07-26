// Package ark 是 AI 上游适配器：Anthropic 兼容聊天接口、方舟 Ark 图片模型、
// 方舟 runtime 视频任务与资源代理拉取。
//
// 原实现集中在 service/ai_service（直读 global.Config() + 包内构建客户端）；
// 新架构改为注入 *config.Provider 的结构体，每个上游能力一个适配器，
// 分别实现 application/ai 声明的端口。上游请求参数、SSE 事件格式与错误文案
// 与原实现逐字一致。
package ark

import (
	"strings"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/ai"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
)

// aiConfig 取当前配置快照中的 AI 段副本，并规范化聊天 BaseURL。
//
// 与原 ai_service.loadConfig 语义一致：每次调用都读最新快照，支持配置热更新。
func aiConfig(provider *config.Provider) (*config.AI, error) {
	root := provider.Get()
	if root == nil {
		return nil, domain.ErrConfigMissing
	}

	source := root.AI
	if source == nil {
		return nil, domain.ErrConfigMissing
	}

	cfg := *source
	cfg.ChatModelBaseURL = domain.NormalizeBaseURL(cfg.ChatModelBaseURL)

	return &cfg, nil
}

// arkTimeout 把配置的超时秒数转成方舟 SDK 所需的可选超时。
func arkTimeout(cfg *config.AI) *time.Duration {
	if cfg == nil || cfg.TimeoutSec <= 0 {
		return nil
	}

	timeout := time.Duration(cfg.TimeoutSec) * time.Second

	return &timeout
}

// doubaoAPIKey 返回去空格后的豆包模型 API Key。
func doubaoAPIKey(cfg *config.AI) string {
	if cfg == nil {
		return ""
	}

	return strings.TrimSpace(cfg.DoubaoModelAPIKey)
}
