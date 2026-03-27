package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"f_ccg_video_back_ddd/internal/config"
	aidomain "f_ccg_video_back_ddd/internal/domain/ai"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type flowTextGenerator struct {
	invoke func(context.Context, TextInvocation) (TextResult, error)
	stream func(context.Context, TextInvocation) (*schema.StreamReader[*schema.Message], error)
}

func (g *flowTextGenerator) InvokeText(ctx context.Context, invocation TextInvocation) (TextResult, error) {
	return g.invoke(ctx, invocation)
}

func (g *flowTextGenerator) OpenStream(ctx context.Context, invocation TextInvocation) (*schema.StreamReader[*schema.Message], error) {
	return g.stream(ctx, invocation)
}

func TestConversationBudgetsRejectBeforeModelInvocation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, input, system      string
		inputLimit, contextLimit int
		want                     error
	}{
		{name: "latest message exceeds default", input: strings.Repeat("你", 24001), want: aidomain.ErrInputTooLong},
		{name: "configured per message", input: "你好世界", inputLimit: 3, want: aidomain.ErrInputTooLong},
		{name: "system included in context", input: "hello", system: "system", contextLimit: 10, want: aidomain.ErrContextTooLong},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := &config.AI{ChatModel: "chat", ChatModelAPIKey: "key", SystemPrompt: tc.system, InputMaxChars: tc.inputLimit, ContextMaxChars: tc.contextLimit}
			calls := 0
			text := &flowTextGenerator{invoke: func(context.Context, TextInvocation) (TextResult, error) {
				calls++
				return TextResult{}, errors.New("must not reach model")
			}}
			s := NewService(infraconfig.NewProvider(&config.Config{AI: cfg}), text, nil, nil, nil, &promptQueryFake{available: true}, nil, zap.NewNop())
			raw, _ := json.Marshal(tc.input)
			_, err := s.CreateResponse(t.Context(), ResponsesRequest{Input: raw})
			if !errors.Is(err, tc.want) || calls != 0 {
				t.Fatalf("error=%v calls=%d, want %v before any model call", err, calls, tc.want)
			}
		})
	}
}

func TestCreateResponsePreservesTruncationAndUsage(t *testing.T) {
	t.Parallel()
	text := &flowTextGenerator{invoke: func(ctx context.Context, inv TextInvocation) (TextResult, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 18*time.Second || time.Until(deadline) < 15*time.Second {
			t.Errorf("configured 17 second budget not propagated: %v", deadline)
		}
		return TextResult{Text: "partial", ResponseMetadata: ResponseMetadata{FinishReason: "max_tokens", Usage: &TokenUsage{InputTokens: 10, OutputTokens: 4096, TotalTokens: 4106}}}, nil
	}}
	s := NewService(infraconfig.NewProvider(&config.Config{AI: &config.AI{ChatModel: "chat", ChatModelAPIKey: "key", TimeoutSec: 17}}), text, nil, nil, nil, nil, nil, zap.NewNop())
	result, err := s.CreateResponse(t.Context(), ResponsesRequest{Input: json.RawMessage(`"hello"`)})
	if err != nil || result.Status != "incomplete" || result.FinishReason != "max_tokens" || result.Usage.TotalTokens != 4106 || result.Output[0].Content[0].Text != "partial" {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestResponseMetadataDoesNotDoubleCountCumulativeUsage(t *testing.T) {
	t.Parallel()
	var metadata ResponseMetadata
	metadata.Observe(&schema.Message{ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 10, TotalTokens: 10}}})
	metadata.Observe(&schema.Message{ResponseMeta: &schema.ResponseMeta{FinishReason: "end_turn", Usage: &schema.TokenUsage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14}}})
	metadata.Observe(&schema.Message{ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14}}})
	if metadata.Status() != "completed" || *metadata.Usage != (TokenUsage{InputTokens: 10, OutputTokens: 4, TotalTokens: 14}) {
		t.Fatalf("metadata=%+v usage=%+v", metadata, metadata.Usage)
	}
}

func TestOpenResponseStreamKeepsContextUntilClose(t *testing.T) {
	t.Parallel()
	var captured context.Context
	reader, writer := schema.Pipe[*schema.Message](1)
	defer writer.Close()
	text := &flowTextGenerator{stream: func(ctx context.Context, _ TextInvocation) (*schema.StreamReader[*schema.Message], error) {
		captured = ctx
		return reader, nil
	}}
	s := NewService(infraconfig.NewProvider(&config.Config{AI: &config.AI{ChatModel: "chat", ChatModelAPIKey: "key"}}), text, nil, nil, nil, nil, nil, zap.NewNop())
	stream, err := s.OpenResponseStream(t.Context(), ResponsesRequest{Input: json.RawMessage(`"hello"`)})
	if err != nil {
		t.Fatal(err)
	}
	if captured.Err() != nil {
		t.Fatal("stream context cancelled before consumption")
	}
	stream.Close()
	stream.Close()
	if !errors.Is(captured.Err(), context.Canceled) {
		t.Fatalf("context not cancelled on Close: %v", captured.Err())
	}
}

func TestToolInstructionsRespectContextBudget(t *testing.T) {
	t.Parallel()
	s := &Service{
		text:    &promptTextFake{text: `{"intent":"find_video","keywords":["Go"]}`},
		prompts: &promptQueryFake{available: true, latest: []uint{7}, inputs: []aidomain.VideoCandidateInput{{VideoID: 7, Title: strings.Repeat("长标题", 200)}}},
		logger:  zap.NewNop(),
	}
	_, _, err := s.prepareResponsesRequest(t.Context(), &config.AI{ChatModel: "chat", ChatModelAPIKey: "key", SystemPrompt: "base", ContextMaxChars: 100}, RuntimeConfig{}, ResponsesRequest{Input: json.RawMessage(`"找 Go 视频"`)})
	if !errors.Is(err, aidomain.ErrContextTooLong) {
		t.Fatalf("tool instructions exceeded the context budget: %v", err)
	}
}
