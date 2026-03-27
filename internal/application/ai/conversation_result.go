package ai

import "github.com/cloudwego/eino/schema"

// TokenUsage 是模型实际报告的用量，不参与计费或额度扣减。
type TokenUsage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	TotalTokens  int `json:"totalTokens"`
}

// ResponseMetadata 在非流式与流式响应中保留相同的结束原因和用量。
type ResponseMetadata struct {
	FinishReason string      `json:"finishReason,omitempty"`
	Usage        *TokenUsage `json:"usage,omitempty"`
}

// Observe 合并 Eino 的累计用量；message_start 与末尾汇总不能重复累加。
func (m *ResponseMetadata) Observe(message *schema.Message) {
	if message == nil || message.ResponseMeta == nil {
		return
	}
	meta := message.ResponseMeta
	if meta.FinishReason != "" {
		m.FinishReason = meta.FinishReason
	}
	if meta.Usage == nil {
		return
	}
	if m.Usage == nil {
		m.Usage = &TokenUsage{}
	}
	m.Usage.InputTokens = max(m.Usage.InputTokens, meta.Usage.PromptTokens)
	m.Usage.OutputTokens = max(m.Usage.OutputTokens, meta.Usage.CompletionTokens)
	m.Usage.TotalTokens = max(m.Usage.TotalTokens, meta.Usage.TotalTokens, m.Usage.InputTokens+m.Usage.OutputTokens)
}

// Status 只有模型明确正常结束时才返回 completed，避免把提前 EOF 当作成功。
func (m ResponseMetadata) Status() string {
	switch m.FinishReason {
	case "end_turn", "stop", "stop_sequence":
		return "completed"
	default:
		return "incomplete"
	}
}

// TextResult 是模型调用结果；文本消费者也必须检查结束状态。
type TextResult struct {
	Text string
	ResponseMetadata
}

// ResponsesPayload 保留现有 output JSON 结构，并补充真实结束状态与用量。
type ResponsesPayload struct {
	Status string `json:"status"`
	Model  string `json:"model"`
	ResponseMetadata
	Output []ResponseOutput `json:"output"`
}

type ResponseOutput struct {
	Type    string            `json:"type"`
	Content []ResponseContent `json:"content"`
}

type ResponseContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
