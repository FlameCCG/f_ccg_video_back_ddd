// Package ark 是 AI 上游适配器：Anthropic 兼容聊天接口、方舟 Ark 图片模型、
// 方舟 runtime 视频任务与资源代理拉取。
//
// 原实现集中在 service/ai_service（直读 global.Config() + 包内构建客户端）；
// 新架构由 application/ai 在公开用例入口捕获运行配置，再按值传给每个上游
// 适配器。上游请求参数、SSE 事件格式与错误文案与原实现逐字一致。
package ark

import (
	"strings"
	"time"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"
)

// arkTimeout 把配置的超时秒数转成方舟 SDK 所需的可选超时。
func arkTimeout(runtime aiapp.RuntimeConfig) *time.Duration {
	if runtime.TimeoutSec <= 0 {
		return nil
	}

	timeout := time.Duration(runtime.TimeoutSec) * time.Second

	return &timeout
}

// doubaoAPIKey 返回去空格后的豆包模型 API Key。
func doubaoAPIKey(runtime aiapp.RuntimeConfig) string {
	return strings.TrimSpace(runtime.DoubaoAPIKey)
}
