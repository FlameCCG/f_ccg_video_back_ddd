package ark

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"
	domain "f_ccg_video_back_ddd/internal/domain/ai"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
)

// messagesPath 上游 Anthropic 兼容消息接口路径。
const messagesPath = "/v1/messages"

// defaultMaxTokens 未指定时的最大输出 token 数。
const defaultMaxTokens = 4096

// 上游消息对象字段名。
const (
	jsonKeyType    = "type"
	jsonKeyContent = "content"
	jsonKeyRole    = "role"
)

// ChatClient 聊天文本生成适配器（上游为 Anthropic 兼容接口，如 DeepSeek）。
type ChatClient struct {
	cfgProvider *config.Provider
}

var _ aiapp.TextGenerator = (*ChatClient)(nil)

// NewChatClient 构造聊天文本生成适配器。
func NewChatClient(cfgProvider *config.Provider) *ChatClient {
	return &ChatClient{cfgProvider: cfgProvider}
}

// InvokeText 非流式生成；返回提取出的文本与上游原始响应体。
func (c *ChatClient) InvokeText(ctx context.Context, inv aiapp.TextInvocation) (string, []byte, error) {
	cfg, err := aiConfig(c.cfgProvider)
	if err != nil {
		return "", nil, err
	}

	payload, err := buildAnthropicMessagePayload(inv, false)
	if err != nil {
		return "", nil, err
	}

	raw, err := doProviderJSONRequest(
		ctx,
		cfg,
		cfg.ChatModelBaseURL,
		cfg.ChatModelAPIKey,
		http.MethodPost,
		messagesPath,
		payload,
		"application/json",
		nil,
	)
	if err != nil {
		return "", nil, err
	}

	text, err := extractAnthropicText(raw)
	if err != nil {
		return "", raw, err
	}

	return text, raw, nil
}

// OpenStream 打开流式响应，并把上游事件翻译为前端使用的 Responses SSE 格式。
func (c *ChatClient) OpenStream(ctx context.Context, inv aiapp.TextInvocation) (*http.Response, error) {
	cfg, err := aiConfig(c.cfgProvider)
	if err != nil {
		return nil, err
	}

	payload, err := buildAnthropicMessagePayload(inv, true)
	if err != nil {
		return nil, err
	}

	resp, err := openProviderJSONStream(
		ctx,
		cfg,
		cfg.ChatModelBaseURL,
		cfg.ChatModelAPIKey,
		http.MethodPost,
		messagesPath,
		payload,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return translateAnthropicStream(resp), nil
}

// anthropicMessageRequest 上游请求体（字段与原实现逐字一致）.
type anthropicMessageRequest struct {
	Model        string                   `json:"model"`
	System       string                   `json:"system,omitempty"`
	Messages     []anthropicMessage       `json:"messages"`
	MaxTokens    int                      `json:"max_tokens,omitempty"`
	Stream       bool                     `json:"stream,omitempty"`
	Thinking     *anthropicThinkingConfig `json:"thinking,omitempty"`
	OutputConfig *anthropicOutputConfig   `json:"output_config,omitempty"`
}

// anthropicThinkingConfig 控制 DeepSeek / Anthropic 兼容思考开关.
type anthropicThinkingConfig struct {
	Type string `json:"type"` // enabled | disabled
}

// anthropicOutputConfig 控制思考强度（DeepSeek: high / max）.
type anthropicOutputConfig struct {
	Effort string `json:"effort,omitempty"`
}

type anthropicMessage struct {
	Role    string                 `json:"role"`
	Content []anthropicContentPart `json:"content"`
}

type anthropicContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type anthropicMessageResponse struct {
	ID      string                         `json:"id"`
	Type    string                         `json:"type"`
	Role    string                         `json:"role"`
	Model   string                         `json:"model"`
	Content []anthropicMessageResponsePart `json:"content"`
}

type anthropicMessageResponsePart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// responsesAPIResponse Responses API 形态的响应（兼容分支）.
type responsesAPIResponse struct {
	Output []responsesOutputItem `json:"output"`
}

type responsesOutputItem struct {
	Type    string                   `json:"type"`
	Content []responsesOutputContent `json:"content"`
}

type responsesOutputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// buildAnthropicMessagePayload 把调用参数翻译为上游请求体。
func buildAnthropicMessagePayload(inv aiapp.TextInvocation, stream bool) (anthropicMessageRequest, error) {
	systemParts := make([]string, 0, 1)

	messages, err := normalizeAnthropicMessages(inv.Input, &systemParts)
	if err != nil {
		return anthropicMessageRequest{}, err
	}

	if len(messages) == 0 {
		return anthropicMessageRequest{}, domain.ErrNoUserMessage
	}

	maxTokens := inv.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	payload := anthropicMessageRequest{
		Model:     inv.Model,
		System:    strings.Join(systemParts, "\n\n"),
		Messages:  mergeAdjacentAnthropicMessages(messages),
		MaxTokens: maxTokens,
		Stream:    stream,
	}

	thinkingOn := inv.Thinking != nil && *inv.Thinking
	if thinkingOn {
		payload.Thinking = &anthropicThinkingConfig{Type: "enabled"}

		if effort := strings.TrimSpace(inv.ThinkingEffort); effort != "" {
			payload.OutputConfig = &anthropicOutputConfig{Effort: effort}
		}
	} else {
		// DeepSeek 默认开启思考，显式关闭以免未配置时浪费 token
		payload.Thinking = &anthropicThinkingConfig{Type: "disabled"}
	}

	return payload, nil
}

// normalizeAnthropicMessages 把客户端 input 翻译为上游消息数组，system 段单独抽出。
func normalizeAnthropicMessages(raw json.RawMessage, systemParts *[]string) ([]anthropicMessage, error) {
	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return []anthropicMessage{{
			Role:    "user",
			Content: []anthropicContentPart{{Type: domain.ContentTypeText, Text: strings.TrimSpace(plain)}},
		}}, nil
	}

	var messages []map[string]any
	if err := json.Unmarshal(raw, &messages); err != nil {
		return nil, fmt.Errorf("解析 input 消息失败: %w", err)
	}

	out := make([]anthropicMessage, 0, len(messages))

	for _, item := range messages {
		role, _ := item[jsonKeyRole].(string)
		role = strings.ToLower(strings.TrimSpace(role))

		parts := normalizeAnthropicContentParts(item[jsonKeyContent])
		if role == domain.RoleSystem {
			if text := joinAnthropicText(parts); text != "" {
				*systemParts = append(*systemParts, text)
			}

			continue
		}

		if role != "assistant" {
			role = "user"
		}

		if len(parts) == 0 {
			continue
		}

		out = append(out, anthropicMessage{Role: role, Content: parts})
	}

	return out, nil
}

// normalizeAnthropicContentParts 递归展开内容块；图片块降级为文本描述。
func normalizeAnthropicContentParts(content any) []anthropicContentPart {
	switch value := content.(type) {
	case string:
		if text := strings.TrimSpace(value); text != "" {
			return []anthropicContentPart{{Type: domain.ContentTypeText, Text: text}}
		}
	case []any:
		parts := make([]anthropicContentPart, 0, len(value))
		for _, item := range value {
			parts = append(parts, normalizeAnthropicContentParts(item)...)
		}

		return parts
	case map[string]any:
		partType, _ := value[jsonKeyType].(string)

		switch strings.TrimSpace(partType) {
		case "", domain.ContentTypeText, "input_text":
			if text, _ := value["text"].(string); strings.TrimSpace(text) != "" {
				return []anthropicContentPart{{Type: domain.ContentTypeText, Text: strings.TrimSpace(text)}}
			}
		case domain.ContentTypeImageURL, "input_image":
			if imageURL := extractImageURL(value); imageURL != "" {
				return []anthropicContentPart{{
					Type: domain.ContentTypeText,
					Text: "用户提供了一张图片，地址：" + imageURL,
				}}
			}
		}
	}

	return nil
}

// extractImageURL 兼容 image_url 的三种写法（字符串、对象、平铺 url）。
func extractImageURL(part map[string]any) string {
	if imageURL, _ := part[domain.ContentTypeImageURL].(string); strings.TrimSpace(imageURL) != "" {
		return strings.TrimSpace(imageURL)
	}

	if imageURL, ok := part[domain.ContentTypeImageURL].(map[string]any); ok {
		if raw, _ := imageURL["url"].(string); strings.TrimSpace(raw) != "" {
			return strings.TrimSpace(raw)
		}
	}

	if raw, _ := part["url"].(string); strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}

	return ""
}

// mergeAdjacentAnthropicMessages 合并相邻同角色消息（上游要求角色交替）。
func mergeAdjacentAnthropicMessages(messages []anthropicMessage) []anthropicMessage {
	if len(messages) < 2 {
		return messages
	}

	merged := make([]anthropicMessage, 0, len(messages))

	for _, msg := range messages {
		if len(merged) > 0 && merged[len(merged)-1].Role == msg.Role {
			merged[len(merged)-1].Content = append(merged[len(merged)-1].Content, msg.Content...)

			continue
		}

		merged = append(merged, msg)
	}

	return merged
}

func joinAnthropicText(parts []anthropicContentPart) string {
	if len(parts) == 0 {
		return ""
	}

	texts := make([]string, 0, len(parts))

	for _, part := range parts {
		if part.Type == domain.ContentTypeText && strings.TrimSpace(part.Text) != "" {
			texts = append(texts, strings.TrimSpace(part.Text))
		}
	}

	return strings.Join(texts, "\n")
}

// extractAnthropicText 从上游响应中提取文本，兼容 Responses API 形态。
func extractAnthropicText(raw []byte) (string, error) {
	var resp anthropicMessageResponse
	if err := json.Unmarshal(raw, &resp); err == nil && len(resp.Content) > 0 {
		texts := make([]string, 0, len(resp.Content))

		for _, part := range resp.Content {
			if part.Type == domain.ContentTypeText && strings.TrimSpace(part.Text) != "" {
				texts = append(texts, strings.TrimSpace(part.Text))
			}
		}

		if len(texts) > 0 {
			return strings.Join(texts, "\n"), nil
		}
	}

	return extractResponseOutputText(raw)
}

// extractResponseOutputText 从 Responses API 形态响应中提取 output_text。
func extractResponseOutputText(raw []byte) (string, error) {
	var resp responsesAPIResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("解析 AI 响应结构失败: %w", err)
	}

	for _, item := range resp.Output {
		if item.Type != "message" {
			continue
		}

		for _, content := range item.Content {
			if content.Type == "output_text" && strings.TrimSpace(content.Text) != "" {
				return strings.TrimSpace(content.Text), nil
			}
		}
	}

	return "", domain.ErrOutputTextNotFound
}
