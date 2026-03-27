package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"
	"f_ccg_video_back_ddd/internal/config"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/external/ark"

	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
)

type availablePromptQuery struct{ aiapp.PromptVideoQuery }

func (availablePromptQuery) Available() bool { return true }

func TestAgentResponseStreamPreservesIncompleteTerminalMetadata(t *testing.T) {
	t.Parallel()
	for _, reason := range []string{"max_tokens", ""} {
		t.Run(reason, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, `event: message_start
data: {"type":"message_start","message":{"id":"msg_partial","type":"message","role":"assistant","model":"test-model","content":[],"usage":{"input_tokens":10,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"partial answer"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

`)
				if reason != "" {
					_, _ = fmt.Fprintf(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":%q},\"usage\":{\"output_tokens\":4096}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", reason)
				}
			}))
			defer server.Close()
			provider := infraconfig.NewProvider(&config.Config{AI: &config.AI{
				ChatModelBaseURL: server.URL, ChatModelAPIKey: "test-key", ChatModel: "test-model",
			}})
			service := aiapp.NewService(provider, ark.NewChatModel(), nil, nil, nil, availablePromptQuery{}, nil, nil)
			stream, err := service.OpenResponseStream(t.Context(), aiapp.ResponsesRequest{Input: json.RawMessage(`"hello"`)})
			if err != nil {
				t.Fatal(err)
			}
			defer stream.Close()
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest("POST", "/responses", nil).WithContext(t.Context())
			NewHandler(service, nil).forwardResponsesStream(c, stream)
			body := recorder.Body.String()
			if !strings.Contains(body, "partial answer") || !strings.Contains(body, `"status":"incomplete"`) || !strings.Contains(body, `"inputTokens":10`) {
				t.Fatalf("incomplete agent response lost text or terminal metadata: %s", body)
			}
			if reason != "" && (!strings.Contains(body, `"finishReason":"max_tokens"`) || !strings.Contains(body, `"outputTokens":4096`)) {
				t.Fatalf("truncation reason or usage was lost: %s", body)
			}
			if strings.Contains(body, "response.completed") || strings.Contains(body, "[DONE]") || strings.Contains(body, `"status":"failed"`) {
				t.Fatalf("incomplete answer reported an incorrect terminal state: %s", body)
			}
		})
	}
}

type responseRecvFunc func() (*schema.Message, error)

func (f responseRecvFunc) Recv() (*schema.Message, error) { return f() }
func (responseRecvFunc) Close()                           {}

func TestForwardResponsesStreamTimeoutDuringReceive(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		requestDeadline bool
		receiveErr      error
	}{
		{name: "request deadline", requestDeadline: true, receiveErr: errors.New("upstream stopped")},
		{name: "request deadline with EOF", requestDeadline: true, receiveErr: io.EOF},
		{name: "upstream deadline", receiveErr: fmt.Errorf("upstream: %w", context.DeadlineExceeded)},
		{name: "transport timeout", receiveErr: &net.OpError{Op: "read", Net: "tcp", Err: os.ErrDeadlineExceeded}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			if tc.requestDeadline {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancel()
			}
			calls := 0
			stream := responseRecvFunc(func() (*schema.Message, error) {
				calls++
				if calls == 1 {
					return &schema.Message{Content: "部分回答", ResponseMeta: &schema.ResponseMeta{
						FinishReason: "end_turn", Usage: &schema.TokenUsage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14},
					}}, nil
				}
				if tc.requestDeadline {
					<-ctx.Done()
				}
				return nil, tc.receiveErr
			})
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest("POST", "/responses", nil).WithContext(ctx)
			NewHandler(nil, nil).forwardResponsesStream(c, stream)
			body := recorder.Body.String()
			if calls != 2 || !strings.Contains(body, `"status":"incomplete"`) || !strings.Contains(body, "AI 回答超时") || !strings.Contains(body, `"totalTokens":14`) {
				t.Fatalf("calls=%d response=%s", calls, body)
			}
			if strings.Contains(body, "response.completed") || strings.Contains(body, "[DONE]") || strings.Contains(body, `"status":"failed"`) {
				t.Fatalf("timeout reported an incorrect terminal state: %s", body)
			}
		})
	}
}

func TestForwardResponsesStreamCancellationDuringReceive(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream := responseRecvFunc(func() (*schema.Message, error) {
		cancel()
		return nil, context.DeadlineExceeded
	})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/responses", nil).WithContext(ctx)
	NewHandler(nil, nil).forwardResponsesStream(c, stream)
	if body := recorder.Body.String(); body != "" {
		t.Fatalf("client cancellation emitted an event: %s", body)
	}
}

func TestWriteResponsesChunk(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := writeResponsesChunk(&output, &schema.Message{
		Role:             schema.Assistant,
		ReasoningContent: "先分析",
		Content:          "最终回答",
	})
	if err != nil {
		t.Fatalf("writeResponsesChunk() error = %v", err)
	}

	const expected = "data: {\"delta\":\"先分析\",\"type\":\"response.reasoning_summary_text.delta\"}\n\n" +
		"data: {\"delta\":\"最终回答\",\"type\":\"response.output_text.delta\"}\n\n"
	if output.String() != expected {
		t.Fatalf("writeResponsesChunk() = %q, want %q", output.String(), expected)
	}
}

func TestWriteToolLifecycleEvent(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	err := writeResponsesChunk(&output, aiapp.ToolEventMessage(aiapp.ToolCallEvent{Phase: "completed", CallID: "toolu_1", Name: "search_videos", Label: "搜索站内视频", Query: "Go教程", ResultCount: 2}))
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Type, CallID, Name, Query string
		ResultCount               int
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(output.String(), "data: "))), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Type != "response.tool.completed" || payload.CallID != "toolu_1" || payload.ResultCount != 2 || payload.Query != "Go教程" {
		t.Fatalf("tool lifecycle not serialized: %+v", payload)
	}
}

func TestForwardResponsesStream(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		streamErr    error
		cancelled    bool
		want         string
		absent       string
		finishReason string
	}{
		{name: "completed", finishReason: "end_turn", want: "response.completed", absent: "response.failed"},
		{name: "truncated", finishReason: "max_tokens", want: "response.failed", absent: "response.completed"},
		{name: "missing terminal reason", want: "response.failed", absent: "response.completed"},
		{name: "upstream failed", streamErr: errors.New("private upstream credentials"), want: "response.failed", absent: "response.completed"},
		{name: "cancelled", cancelled: true, absent: "data:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stream, writer := schema.Pipe[*schema.Message](2)
			defer stream.Close()
			writer.Send(&schema.Message{Content: "部分回答", ResponseMeta: &schema.ResponseMeta{FinishReason: tc.finishReason}}, nil)
			if tc.streamErr != nil {
				writer.Send(nil, tc.streamErr)
			}
			writer.Close()
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			c.Request = httptest.NewRequest("POST", "/responses", nil).WithContext(ctx)
			NewHandler(nil, nil).forwardResponsesStream(c, stream)
			body := recorder.Body.String()
			if tc.finishReason == "max_tokens" && (!strings.Contains(body, `"status":"incomplete"`) || !strings.Contains(body, `"finishReason":"max_tokens"`)) {
				t.Fatalf("truncation metadata lost: %q", body)
			}
			if tc.want != "" && !strings.Contains(body, tc.want) {
				t.Fatalf("response = %q, want %q", body, tc.want)
			}
			if strings.Contains(body, tc.absent) || strings.Contains(body, "private upstream credentials") {
				t.Fatalf("response contains forbidden event or private error: %q", body)
			}
		})
	}
}

func TestWriteResponsesCompleted(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	if err := writeResponsesCompleted(&output, aiapp.ResponseMetadata{}); err != nil {
		t.Fatalf("writeResponsesCompleted() error = %v", err)
	}

	const expected = "data: {\"type\":\"response.completed\"}\n\ndata: [DONE]\n\n"
	if output.String() != expected {
		t.Fatalf("writeResponsesCompleted() = %q, want %q", output.String(), expected)
	}
}
