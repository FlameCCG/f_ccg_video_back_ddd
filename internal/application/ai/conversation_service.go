package ai

import (
	"context"
	"time"

	"f_ccg_video_back_ddd/internal/config"
	aidomain "f_ccg_video_back_ddd/internal/domain/ai"

	"go.uber.org/zap"
)

// defaultMaxOutputTokens 聊天正式回答的最大输出 token 数。
const defaultMaxOutputTokens = 4096

// CreateResponse 调用聊天模型生成非流式文本响应.
func (s *Service) CreateResponse(ctx context.Context, req ResponsesRequest) (*ResponsesPayload, error) {
	snapshot, err := s.snapshot()
	if err != nil {
		return nil, err
	}
	ctx, cancel := snapshot.runtime.WithContext(ctx)
	defer cancel()
	prepared, conv, err := s.prepareResponsesRequest(ctx, &snapshot.settings, snapshot.runtime, req)
	if err != nil {
		return nil, err
	}
	result, err := s.invokeText(ctx, TextInvocation{
		Runtime:         snapshot.runtime,
		Model:           prepared.Model,
		Messages:        conv.modelMessages(),
		Thinking:        prepared.Thinking,
		ThinkingEffort:  prepared.ThinkingEffort,
		MaxTokens:       defaultMaxOutputTokens,
		Tools:           s.conversationTools(&snapshot.settings, conv),
		ContextMaxChars: snapshot.settings.ContextMaxChars,
	})
	if err != nil {
		return nil, err
	}
	return buildResponsesCompatiblePayload(prepared.Model, result), nil
}

// OpenResponseStream 打开聊天模型流式响应.
//
// 返回流的消费与关闭由接口层负责，客户端断开需沿 context 传播到上游，避免留下无人读取的生成请求。
// 本用例不直接操作 Gin Writer；HTTP/SSE 编码在接口和外部模型适配器中处理。
func (s *Service) OpenResponseStream(ctx context.Context, req ResponsesRequest) (MessageStream, error) {
	started := time.Now()
	snapshot, err := s.snapshot()
	if err != nil {
		return nil, err
	}
	ctx, cancel := snapshot.runtime.WithContext(ctx)
	prepared, conv, err := s.prepareResponsesRequest(ctx, &snapshot.settings, snapshot.runtime, req)
	if err != nil {
		cancel()
		return nil, err
	}
	stream, err := s.text.OpenStream(ctx, TextInvocation{
		Runtime:         snapshot.runtime,
		Model:           prepared.Model,
		Messages:        conv.modelMessages(),
		Thinking:        prepared.Thinking,
		ThinkingEffort:  prepared.ThinkingEffort,
		MaxTokens:       defaultMaxOutputTokens,
		Tools:           s.conversationTools(&snapshot.settings, conv),
		ContextMaxChars: snapshot.settings.ContextMaxChars,
	})
	if err != nil {
		cancel()
		return nil, err
	}
	if stream == nil {
		cancel()
		return nil, aidomain.ErrOutputIncomplete
	}
	return &responseStream{
		source: stream,
		cancel: cancel,
		start:  started,
		logger: s.log().With(zap.String("model", prepared.Model)),
	}, nil
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

	if !conv.hasInput(req.Input) {
		return req, conv, aidomain.ErrEmptyInput
	}

	var err error
	req.Model, err = aidomain.Options(toOptions(cfg.ChatModels)).NormalizeConfiguredModel(req.Model, cfg.ChatModel, "聊天")
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
	if err = conv.validateInput(cfg); err != nil {
		return req, conv, err
	}
	conv = conv.trimHistory(cfg)
	if !conv.HasMessages() {
		return req, conv, aidomain.ErrNoUserMessage
	}
	base := conv.prependSystem(cfg.SystemPrompt)
	if err = base.validateContext(cfg); err != nil {
		return req, conv, err
	}

	prompt := cfg.SystemPrompt
	if s.prompts != nil && s.prompts.Available() {
		prompt += "\n\n" + videoSearchInstruction
	}
	enhanced := conv.prependSystem(prompt)
	if err = enhanced.validateContext(cfg); err != nil {
		return req, conv, err
	}
	conv = enhanced

	return req, conv, nil
}

// normalizeThinkingOptions 按后台开关规范化本次请求的思考选项。
func normalizeThinkingOptions(cfg *config.AI, req ResponsesRequest) (ResponsesRequest, error) {
	var enabled bool
	var configured string
	var options aidomain.Options
	if cfg != nil {
		enabled, configured = cfg.ThinkingEnabled, cfg.ThinkingEffort
		options = toOptions(cfg.ThinkingEfforts)
	}
	selection, err := options.ResolveThinking(enabled, req.Thinking, req.ThinkingEffort, configured)
	if err != nil {
		return req, err
	}
	req.Thinking, req.ThinkingEffort = &selection.Enabled, selection.Effort
	return req, nil
}

// buildResponsesCompatiblePayload 组装 Responses API 兼容的非流式响应载荷。
func buildResponsesCompatiblePayload(model string, result TextResult) *ResponsesPayload {
	return &ResponsesPayload{
		Status:           result.Status(),
		Model:            model,
		ResponseMetadata: result.ResponseMetadata,
		Output: []ResponseOutput{{
			Type:    "message",
			Content: []ResponseContent{{Type: "output_text", Text: result.Text}},
		}},
	}
}
