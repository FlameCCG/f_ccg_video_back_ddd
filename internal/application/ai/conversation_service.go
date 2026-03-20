package ai

import (
	"context"

	"f_ccg_video_back_ddd/internal/config"
	aierrors "f_ccg_video_back_ddd/internal/domain/ai/domainerr"
	aivo "f_ccg_video_back_ddd/internal/domain/ai/valueobject"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// defaultMaxOutputTokens 聊天正式回答的最大输出 token 数。
const defaultMaxOutputTokens = 4096

// CreateResponse 调用聊天模型生成非流式文本响应.
func (s *Service) CreateResponse(ctx context.Context, req ResponsesRequest) (any, error) {
	snapshot, err := s.snapshot()
	if err != nil {
		return nil, err
	}

	prepared, conv, err := s.prepareResponsesRequest(ctx, &snapshot.settings, snapshot.runtime, req)
	if err != nil {
		return nil, err
	}

	text, err := s.text.InvokeText(ctx, TextInvocation{
		Runtime:        snapshot.runtime,
		Model:          prepared.Model,
		Messages:       conv.modelMessages(),
		Thinking:       prepared.Thinking,
		ThinkingEffort: prepared.ThinkingEffort,
		MaxTokens:      defaultMaxOutputTokens,
	})
	if err != nil {
		return nil, err
	}

	return buildResponsesCompatiblePayload(prepared.Model, text), nil
}

// OpenResponseStream 打开聊天模型流式响应.
//
// 返回流的消费与关闭由接口层负责，客户端断开需沿 context 传播到上游，避免留下无人读取的生成请求。
// 本用例不直接操作 Gin Writer；HTTP/SSE 编码在接口和外部模型适配器中处理。
func (s *Service) OpenResponseStream(
	ctx context.Context,
	req ResponsesRequest,
) (*schema.StreamReader[*schema.Message], error) {
	snapshot, err := s.snapshot()
	if err != nil {
		return nil, err
	}

	prepared, conv, err := s.prepareResponsesRequest(ctx, &snapshot.settings, snapshot.runtime, req)
	if err != nil {
		return nil, err
	}

	return s.text.OpenStream(ctx, TextInvocation{
		Runtime:        snapshot.runtime,
		Model:          prepared.Model,
		Messages:       conv.modelMessages(),
		Thinking:       prepared.Thinking,
		ThinkingEffort: prepared.ThinkingEffort,
		MaxTokens:      defaultMaxOutputTokens,
	})
}

// prepareResponsesRequest 校验并补全聊天请求：模型白名单、思考选项与系统提示词注入。
func (s *Service) prepareResponsesRequest(
	ctx context.Context,
	cfg *config.AI,
	runtime RuntimeConfig,
	req ResponsesRequest,
) (ResponsesRequest, conversation, error) {
	var conv conversation

	if err := requireChatAPIKey(cfg); err != nil {
		return req, conv, err
	}

	if !aivo.HasRawJSON(req.Input) {
		return req, conv, aierrors.ErrEmptyInput
	}

	var err error
	req.Model, err = aivo.NormalizeConfiguredModel(req.Model, cfg.ChatModel, toOptions(cfg.ChatModels), "聊天")
	if err != nil {
		return req, conv, err
	}

	req, err = normalizeThinkingOptions(cfg, req)
	if err != nil {
		return req, conv, err
	}

	conv, err = parseConversation(req.Input)
	if err != nil {
		return req, conv, err
	}
	conv = conv.trimHistory(cfg)
	if !conv.HasMessages() {
		return req, conv, aierrors.ErrNoUserMessage
	}

	prompt, promptErr := s.buildResponsesSystemPrompt(ctx, cfg, runtime, conv, req.Model)
	if promptErr != nil {
		s.logger.Warn("build AI responses system prompt failed", zap.Error(promptErr))
	}

	conv = conv.prependSystem(aivo.FirstNonEmpty(prompt, cfg.SystemPrompt))

	return req, conv, nil
}

// normalizeThinkingOptions 按后台开关规范化本次请求的思考选项。
func normalizeThinkingOptions(cfg *config.AI, req ResponsesRequest) (ResponsesRequest, error) {
	if cfg == nil || !cfg.ThinkingEnabled {
		disabled := false
		req.Thinking = &disabled
		req.ThinkingEffort = ""

		return req, nil
	}

	// 未传 thinking 时默认开启（与后台开关一致）
	if req.Thinking == nil {
		enabled := true
		req.Thinking = &enabled
	}

	if req.Thinking != nil && !*req.Thinking {
		req.ThinkingEffort = ""

		return req, nil
	}

	effort, err := aivo.NormalizeConfiguredOption(
		req.ThinkingEffort,
		cfg.ThinkingEffort,
		toOptions(cfg.ThinkingEfforts),
		"思考强度",
		"high",
	)
	if err != nil {
		return req, err
	}

	req.ThinkingEffort = effort

	return req, nil
}

// buildResponsesCompatiblePayload 组装 Responses API 兼容的非流式响应载荷。
func buildResponsesCompatiblePayload(model, text string) map[string]any {
	return map[string]any{
		"status": "completed",
		"model":  model,
		"output": []map[string]any{
			{
				"type": "message",
				"content": []map[string]any{
					{
						"type": "output_text",
						"text": text,
					},
				},
			},
		},
	}
}
