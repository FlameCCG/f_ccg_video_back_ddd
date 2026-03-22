package ai

import (
	"encoding/json"

	aidomain "f_ccg_video_back_ddd/internal/domain/ai"
)

// ResponsesRequest 表示聊天/文本生成请求，也是路由层泛型参数绑定的目标。
type ResponsesRequest struct {
	Model string          `json:"model,omitempty"`
	Input json.RawMessage `json:"input" binding:"required"`
	// Thinking 为本次请求是否开启思考；nil 时跟随后台 thinkingEnabled.
	// 仅当后台 thinkingEnabled=true 时，客户端可将此设为 false 以关闭思考.
	Thinking *bool `json:"thinking,omitempty"`
	// ThinkingEffort 思考强度（如 high / max）；空则用后台默认.
	ThinkingEffort string `json:"thinkingEffort,omitempty"`
}

// PublicOptions 前台 AI 模型与思考配置（不含密钥）。
//
// 选项列表元素为 domain.Option，以 label/value 字段返回选项。
type PublicOptions struct {
	ChatModel       string            `json:"chatModel"`
	ChatModels      []aidomain.Option `json:"chatModels"`
	ThinkingEnabled bool              `json:"thinkingEnabled"`
	ThinkingEffort  string            `json:"thinkingEffort"`
	ThinkingEfforts []aidomain.Option `json:"thinkingEfforts"`
	ImageModel      string            `json:"imageModel"`
	ImageModels     []aidomain.Option `json:"imageModels"`
	VideoModel      string            `json:"videoModel"`
	VideoModels     []aidomain.Option `json:"videoModels"`
}
