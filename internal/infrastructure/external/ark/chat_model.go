package ark

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/pkg/apperr"
	"fmt"
	"net/http"
	"strings"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/schema"
)

const defaultChatMaxTokens = 4096

// ChatModel 使用 Eino Claude 组件调用 Anthropic Messages 兼容接口。
type ChatModel struct{}

var _ aiapp.TextGenerator = (*ChatModel)(nil)

// NewChatModel 构造聊天模型适配器；运行配置由每次调用显式传入。
func NewChatModel() *ChatModel {
	return &ChatModel{}
}

// InvokeText 非流式生成文本。
func (m *ChatModel) InvokeText(ctx context.Context, invocation aiapp.TextInvocation) (string, error) {
	if len(invocation.Messages) == 0 {
		return "", errors.New("input 中没有可发送的用户消息")
	}

	model, err := m.newModel(ctx, invocation, httpClientFor(invocation.Runtime, false))
	if err != nil {
		return "", err
	}

	response, err := model.Generate(ctx, invocation.Messages)
	if err != nil {
		return "", apperr.Private("ai.generate", fmt.Errorf("eino 聊天模型生成失败: %w", err))
	}

	if response == nil {
		return "", errors.New("eino 聊天模型返回空响应")
	}

	return strings.TrimSpace(response.Content), nil
}

// OpenStream 打开 Eino 消息流。
func (m *ChatModel) OpenStream(
	ctx context.Context,
	invocation aiapp.TextInvocation,
) (*schema.StreamReader[*schema.Message], error) {
	if len(invocation.Messages) == 0 {
		return nil, errors.New("input 中没有可发送的用户消息")
	}

	httpClient, releaseResponseBody := gatedStreamHTTPClient()

	model, err := m.newModel(ctx, invocation, httpClient)
	if err != nil {
		releaseResponseBody()

		return nil, err
	}

	stream, err := model.Stream(ctx, invocation.Messages)

	releaseResponseBody()

	if err != nil {
		return nil, apperr.Private("ai.stream.open", fmt.Errorf("打开 Eino 聊天模型流失败: %w", err))
	}

	return stream, nil
}

func (m *ChatModel) newModel(
	ctx context.Context,
	invocation aiapp.TextInvocation,
	httpClient *http.Client,
) (*claude.ChatModel, error) {
	baseURL := strings.TrimSpace(invocation.Runtime.ChatModelBaseURL)
	if baseURL == "" {
		return nil, errors.New("ai 聊天模型 Base URL 未配置")
	}

	maxTokens := invocation.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultChatMaxTokens
	}

	thinkingType := "disabled"
	if invocation.Thinking != nil && *invocation.Thinking {
		thinkingType = "enabled"
	}

	additional := map[string]any{
		"thinking": map[string]any{"type": thinkingType},
	}
	if effort := strings.TrimSpace(invocation.ThinkingEffort); thinkingType == "enabled" && effort != "" {
		additional["output_config"] = map[string]any{"effort": effort}
	}

	model, err := claude.NewChatModel(ctx, &claude.Config{
		BaseURL:                 &baseURL,
		APIKey:                  strings.TrimSpace(invocation.Runtime.ChatModelAPIKey),
		Model:                   strings.TrimSpace(invocation.Model),
		MaxTokens:               maxTokens,
		HTTPClient:              httpClient,
		AdditionalRequestFields: additional,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 Eino 聊天模型失败: %w", err)
	}

	return model, nil
}
