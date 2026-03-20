package ai

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// TextInvocation 聊天模型调用参数（已完成模型/思考选项校验与系统提示词注入）。
type TextInvocation struct {
	Runtime RuntimeConfig
	Model   string
	// Messages 是解析、裁剪并注入服务端系统提示词后的 Eino 标准消息。
	Messages []*schema.Message
	// Thinking 是否开启思考；nil 视为关闭。
	Thinking *bool
	// ThinkingEffort 思考强度，空表示不下发。
	ThinkingEffort string
	// MaxTokens 最大输出 token 数。
	MaxTokens int
}

// TextGenerator 聊天文本生成端口（上游为 Anthropic 兼容接口）。
//
// 实现见 infrastructure/external/ark：负责上游请求体拼装、SSE 事件翻译与错误映射。
type TextGenerator interface {
	// InvokeText 非流式生成。
	InvokeText(ctx context.Context, inv TextInvocation) (string, error)
	// OpenStream 打开 Eino 消息流；调用方负责 Close。
	OpenStream(ctx context.Context, inv TextInvocation) (*schema.StreamReader[*schema.Message], error)
}
