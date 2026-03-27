package ark

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"
	aidomain "f_ccg_video_back_ddd/internal/domain/ai"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const maxConversationSearches = 2

// incompleteAgentRoundError 阻止 Agent 继续执行工具，同时保留可交付的部分回答。
// 错误携带独立快照，避免后续流读取改变已经观察到的终态与用量。
type incompleteAgentRoundError struct {
	result aiapp.TextResult
}

func newIncompleteAgentRoundError(text string, metadata aiapp.ResponseMetadata) *incompleteAgentRoundError {
	if metadata.Usage != nil {
		usage := *metadata.Usage
		metadata.Usage = &usage
	}
	return &incompleteAgentRoundError{result: aiapp.TextResult{Text: text, ResponseMetadata: metadata}}
}

func (*incompleteAgentRoundError) Error() string { return aidomain.ErrOutputIncomplete.Error() }
func (*incompleteAgentRoundError) Unwrap() error { return aidomain.ErrOutputIncomplete }

// conversationModel 为 Agent 的每一轮创建独立读闸门，协调 Claude 组件的并发读取。
// 模型上下文和调用选项原样传递，包括思考签名和工具消息配对。
type conversationModel struct {
	adapter *ChatModel
	inv     aiapp.TextInvocation
}

func (m *conversationModel) validate(messages []*schema.Message) error {
	chars := 0
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		chars += utf8.RuneCountInString(msg.Content) + utf8.RuneCountInString(msg.ReasoningContent)
		for _, call := range msg.ToolCalls {
			chars += utf8.RuneCountInString(call.Function.Arguments)
		}
	}
	return (aidomain.InputPolicy{}).ValidateContext(chars, m.inv.ContextMaxChars)
}

func (m *conversationModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if err := m.validate(messages); err != nil {
		return nil, err
	}
	cm, err := m.adapter.newModel(ctx, m.inv, httpClientFor(m.inv.Runtime))
	if err != nil {
		return nil, err
	}
	response, err := cm.Generate(ctx, messages, opts...)
	if err == nil {
		var metadata aiapp.ResponseMetadata
		metadata.Observe(response)
		if validateAgentRound(response) != nil || metadata.FinishReason == "" {
			text := ""
			if response != nil {
				text = response.Content
			}
			err = newIncompleteAgentRoundError(text, metadata)
		}
	}
	return response, err
}

func (m *conversationModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if err := m.validate(messages); err != nil {
		return nil, err
	}
	client, release := gatedStreamHTTPClient(m.inv.Runtime)
	defer release()
	cm, err := m.adapter.newModel(ctx, m.inv, client)
	if err != nil {
		return nil, err
	}
	source, err := cm.Stream(ctx, messages, opts...)
	if err != nil {
		return nil, err
	}
	var metadata aiapp.ResponseMetadata
	var text strings.Builder
	return schema.StreamReaderWithConvert(source, func(msg *schema.Message) (*schema.Message, error) {
		metadata.Observe(msg)
		if msg != nil {
			text.WriteString(msg.Content)
		}
		if err := validateAgentRound(msg); err != nil {
			return msg, newIncompleteAgentRoundError(text.String(), metadata)
		}
		return msg, nil
	}, schema.WithOnEOF(func() (any, error) {
		if metadata.FinishReason == "" {
			return nil, newIncompleteAgentRoundError(text.String(), metadata)
		}
		return nil, io.EOF
	})), nil
}

// 截断即使碰巧形成合法 JSON 也不能执行工具。空原因允许增量继续到达。
func validateAgentRound(msg *schema.Message) error {
	if msg == nil || msg.ResponseMeta == nil {
		return nil
	}
	switch msg.ResponseMeta.FinishReason {
	case "", "end_turn", "stop", "stop_sequence", "tool_use", "tool_calls":
		return nil
	default:
		return aidomain.ErrOutputIncomplete
	}
}

type observedConversationTool struct {
	inner   tool.InvokableTool
	calls   *atomic.Int32
	publish func(*schema.Message) error
}

func (t *observedConversationTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.inner.Info(ctx)
}

func (t *observedConversationTool) InvokableRun(ctx context.Context, arguments string, opts ...tool.Option) (string, error) {
	info, err := t.inner.Info(ctx)
	if err != nil {
		return "", err
	}
	var args struct {
		Query string `json:"query"`
	}
	_ = json.Unmarshal([]byte(arguments), &args)
	event := aiapp.ToolCallEvent{CallID: compose.GetToolCallID(ctx), Name: info.Name, Label: "搜索站内视频", Query: args.Query}
	emit := func(phase string) error {
		event.Phase = phase
		if t.publish != nil {
			return t.publish(aiapp.ToolEventMessage(event))
		}
		return nil
	}
	if err := emit("started"); err != nil {
		return "", err
	}
	var result string
	if t.calls.Add(1) > maxConversationSearches {
		err = errors.New("search invocation limit reached")
		event.Message = "本轮搜索次数已达到上限"
	} else {
		result, err = t.inner.InvokableRun(ctx, arguments, opts...)
	}
	if err != nil {
		if event.Message == "" {
			event.Message = "站内搜索暂时未能完成"
		}
		if emitErr := emit("failed"); emitErr != nil {
			return "", emitErr
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		// 保留失败的工具结果供模型解释或改写搜索，不泄露内部错误。
		raw, _ := json.Marshal(map[string]string{"error": event.Message})
		return string(raw), nil
	}
	var summary struct {
		ResultCount   int    `json:"resultCount"`
		RetrievalMode string `json:"retrievalMode"`
	}
	_ = json.Unmarshal([]byte(result), &summary)
	event.ResultCount, event.RetrievalMode = summary.ResultCount, summary.RetrievalMode
	if err := emit("completed"); err != nil {
		return "", err
	}
	return result, nil
}

func (m *ChatModel) openAgentStream(parent context.Context, inv aiapp.TextInvocation) (*schema.StreamReader[*schema.Message], error) {
	ctx, cancel := context.WithCancel(parent)
	reader, writer := schema.Pipe[*schema.Message](8)
	go func() {
		defer cancel()
		defer writer.Close()
		publish := func(message *schema.Message) error {
			if writer.Send(message, nil) {
				return context.Canceled
			}
			return nil
		}
		result, err := m.runAgent(ctx, inv, true, publish)
		if err != nil {
			writer.Send(nil, err)
			return
		}
		usage := result.Usage
		meta := &schema.ResponseMeta{FinishReason: result.FinishReason}
		if usage != nil {
			meta.Usage = &schema.TokenUsage{PromptTokens: usage.InputTokens, CompletionTokens: usage.OutputTokens, TotalTokens: usage.TotalTokens}
		}
		_ = publish(&schema.Message{Role: schema.Assistant, ResponseMeta: meta})
	}()
	return reader, nil
}

func (m *ChatModel) runAgent(ctx context.Context, inv aiapp.TextInvocation, streaming bool, publish func(*schema.Message) error) (aiapp.TextResult, error) {
	var calls atomic.Int32
	tools := make([]tool.BaseTool, 0, len(inv.Tools))
	for _, base := range inv.Tools {
		invokable, ok := base.(tool.InvokableTool)
		if !ok {
			return aiapp.TextResult{}, errors.New("conversation tool is not invokable")
		}
		tools = append(tools, &observedConversationTool{inner: invokable, calls: &calls, publish: publish})
	}
	chatAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "video_assistant",
		Model:         &conversationModel{adapter: m, inv: inv},
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools, ExecuteSequentially: true}},
		MaxIterations: maxConversationSearches + 1,
	})
	if err != nil {
		return aiapp.TextResult{}, err
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: chatAgent, EnableStreaming: streaming})
	events := runner.Run(ctx, inv.Messages)
	var result aiapp.TextResult
	total := aiapp.TokenUsage{}
	finishError := func(err error) (aiapp.TextResult, error) {
		var incomplete *incompleteAgentRoundError
		if !errors.As(err, &incomplete) {
			return result, err
		}
		partial := incomplete.result
		if partial.Usage != nil {
			total.InputTokens += partial.Usage.InputTokens
			total.OutputTokens += partial.Usage.OutputTokens
			total.TotalTokens += partial.Usage.TotalTokens
		}
		partial.Usage = &total
		return partial, nil
	}
	previousText := false
	for event, ok := events.Next(); ok; event, ok = events.Next() {
		if event.Err != nil {
			return finishError(event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		output := event.Output.MessageOutput
		if output.Role != schema.Assistant {
			if output.MessageStream != nil {
				output.MessageStream.Close()
			}
			continue
		}
		var round aiapp.ResponseMetadata
		var text strings.Builder
		firstText := true
		onMessage := func(msg *schema.Message) error {
			if msg == nil {
				return nil
			}
			round.Observe(msg)
			text.WriteString(msg.Content)
			if publish == nil || (msg.Content == "" && msg.ReasoningContent == "") {
				return nil
			}
			if firstText && msg.Content != "" {
				if previousText {
					if err := publish(&schema.Message{Content: "\n\n"}); err != nil {
						return err
					}
				}
				firstText = false
			}
			return publish(&schema.Message{Role: schema.Assistant, Content: msg.Content, ReasoningContent: msg.ReasoningContent})
		}
		if output.IsStreaming {
			err = consumeAgentMessages(output.MessageStream, onMessage)
		} else {
			err = onMessage(output.Message)
		}
		if err != nil {
			return finishError(err)
		}
		if round.FinishReason == "" {
			return result, aidomain.ErrOutputIncomplete
		}
		if round.Usage != nil {
			total.InputTokens += round.Usage.InputTokens
			total.OutputTokens += round.Usage.OutputTokens
			total.TotalTokens += round.Usage.TotalTokens
		}
		result = aiapp.TextResult{Text: text.String(), ResponseMetadata: round}
		previousText = previousText || text.Len() > 0
	}
	if result.Status() != "completed" {
		return result, aidomain.ErrOutputIncomplete
	}
	result.Usage = &total
	return result, nil
}

func consumeAgentMessages(reader *schema.StreamReader[*schema.Message], consume func(*schema.Message) error) error {
	defer reader.Close()
	for {
		message, err := reader.Recv()
		// 终态消息与中断错误可在同一次 Recv 返回；先交付该消息的文本和元数据。
		if message != nil {
			if consumeErr := consume(message); consumeErr != nil {
				return consumeErr
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
