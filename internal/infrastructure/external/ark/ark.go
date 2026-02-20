// Package ark 提供 Anthropic 兼容聊天、方舟 Ark 图片模型、runtime 视频任务与资源代理适配器。
//
// 应用用例入口捕获运行配置，并按值传给上游适配器；请求参数、SSE 事件与错误文案属于上游协议。
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
