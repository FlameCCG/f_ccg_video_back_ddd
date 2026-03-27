package ark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type agentSearchFake struct {
	calls atomic.Int32
	fail  bool
	run   func(context.Context, string) (string, error)
}

func (*agentSearchFake) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: aiapp.VideoSearchToolName, Desc: "search videos", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"query": {Type: schema.String, Required: true},
	})}, nil
}

func (s *agentSearchFake) InvokableRun(ctx context.Context, arguments string, _ ...tool.Option) (string, error) {
	s.calls.Add(1)
	if s.run != nil {
		return s.run(ctx, arguments)
	}
	if s.fail {
		return "", errors.New("private database password")
	}
	var input struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(arguments), &input); err != nil || input.Query != "Go 教程" {
		return "", errors.New("invalid arguments")
	}
	return `{"resultCount":2,"retrievalMode":"vector_match","candidates":[{"video_id":7,"route":"/video/7"}]}`, nil
}

func TestAgentNonStreamingUsesToolResultsAndPreservesUsage(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	search := &agentSearchFake{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		round := requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		content := []any{map[string]any{"type": "text", "text": "找到教程。"}}
		reason := "end_turn"
		if round == 1 {
			content = []any{map[string]any{"type": "tool_use", "id": "toolu_search_1", "name": aiapp.VideoSearchToolName, "input": map[string]string{"query": "Go 教程"}}}
			reason = "tool_use"
		} else {
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "tool_result") {
				t.Error("missing tool result")
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "msg_nonstream", "type": "message", "role": "assistant", "model": "test-model", "content": content, "stop_reason": reason, "usage": map[string]int{"input_tokens": 10, "output_tokens": 3}})
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	result, err := NewChatModel().InvokeText(ctx, aiapp.TextInvocation{Runtime: aiapp.RuntimeConfig{ChatModelBaseURL: server.URL, ChatModelAPIKey: "test-key"}, Model: "test-model", Messages: []*schema.Message{schema.UserMessage("找教程")}, Tools: []tool.BaseTool{search}})
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 || search.calls.Load() != 1 || result.Text != "找到教程。" || result.Status() != "completed" || result.Usage.InputTokens != 20 {
		t.Fatalf("result=%+v requests=%d calls=%d", result, requests.Load(), search.calls.Load())
	}
}

func TestAgentPreservesTruncatedTextAndAggregatesUsage(t *testing.T) {
	t.Parallel()
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprintf("streaming=%v", streaming), func(t *testing.T) {
			t.Parallel()
			var requests atomic.Int32
			search := &agentSearchFake{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				first := requests.Add(1) == 1
				if streaming {
					if first {
						startAgentReply(w, 10)
						agentToolCall(w)
						finishAgentReply(w, "tool_use", 5)
					} else {
						startAgentReply(w, 20)
						agentText(w, "partial answer")
						finishAgentReply(w, "max_tokens", 4096)
					}
					return
				}
				w.Header().Set("Content-Type", "application/json")
				content := []any{map[string]any{"type": "text", "text": "partial answer"}}
				reason, input, output := "max_tokens", 20, 4096
				if first {
					content = []any{map[string]any{"type": "tool_use", "id": "toolu_search_1", "name": aiapp.VideoSearchToolName, "input": map[string]string{"query": "Go 教程"}}}
					reason, input, output = "tool_use", 10, 5
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "msg_partial", "type": "message", "role": "assistant", "model": "test-model", "content": content, "stop_reason": reason, "usage": map[string]int{"input_tokens": input, "output_tokens": output}})
			}))
			defer server.Close()
			inv := aiapp.TextInvocation{Runtime: aiapp.RuntimeConfig{ChatModelBaseURL: server.URL, ChatModelAPIKey: "test-key"}, Model: "test-model", Messages: []*schema.Message{schema.UserMessage("找教程")}, Tools: []tool.BaseTool{search}}
			var result aiapp.TextResult
			if streaming {
				reader, err := NewChatModel().OpenStream(t.Context(), inv)
				if err != nil {
					t.Fatal(err)
				}
				defer reader.Close()
				for {
					msg, err := reader.Recv()
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					result.Text += msg.Content
					result.Observe(msg)
				}
			} else {
				var err error
				result, err = NewChatModel().InvokeText(t.Context(), inv)
				if err != nil {
					t.Fatal(err)
				}
			}
			if requests.Load() != 2 || search.calls.Load() != 1 || result.Text != "partial answer" || result.Status() != "incomplete" || result.FinishReason != "max_tokens" || result.Usage == nil || *result.Usage != (aiapp.TokenUsage{InputTokens: 30, OutputTokens: 4101, TotalTokens: 4131}) {
				t.Fatalf("truncated answer or cumulative usage lost: result=%+v usage=%+v requests=%d tools=%d", result, result.Usage, requests.Load(), search.calls.Load())
			}
		})
	}
}

func TestAgentCancellationStopsRunningSearch(t *testing.T) {
	t.Parallel()
	stopped := make(chan struct{})
	search := &agentSearchFake{run: func(ctx context.Context, _ string) (string, error) {
		<-ctx.Done()
		close(stopped)
		return "", ctx.Err()
	}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		startAgentReply(w, 10)
		agentToolCall(w)
		finishAgentReply(w, "tool_use", 5)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	reader, err := NewChatModel().OpenStream(ctx, aiapp.TextInvocation{Runtime: aiapp.RuntimeConfig{ChatModelBaseURL: server.URL, ChatModelAPIKey: "test-key"}, Model: "test-model", Messages: []*schema.Message{schema.UserMessage("找教程")}, Tools: []tool.BaseTool{search}})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	for {
		message, err := reader.Recv()
		if err != nil {
			t.Fatal(err)
		}
		if event, ok := aiapp.MessageToolEvent(message); ok && event.Phase == "started" {
			cancel()
			break
		}
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("running tool ignored cancellation")
	}
}

func TestAgentToolFailureIsVisibleWithoutLeakingUpstreamError(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	search := &agentSearchFake{fail: true}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			startAgentReply(w, 10)
			agentToolCall(w)
			finishAgentReply(w, "tool_use", 5)
		} else {
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), "private database password") {
				t.Error("private error leaked into model context")
			}
			startAgentReply(w, 20)
			agentText(w, "搜索暂时不可用。")
			finishAgentReply(w, "end_turn", 4)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	reader, err := NewChatModel().OpenStream(ctx, aiapp.TextInvocation{Runtime: aiapp.RuntimeConfig{ChatModelBaseURL: server.URL, ChatModelAPIKey: "test-key"}, Model: "test-model", Messages: []*schema.Message{schema.UserMessage("找教程")}, Tools: []tool.BaseTool{search}})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	failed := false
	for {
		message, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if event, ok := aiapp.MessageToolEvent(message); ok && event.Phase == "failed" {
			failed = true
			if strings.Contains(event.Message, "private") {
				t.Fatal("private error leaked into SSE")
			}
		}
	}
	if !failed || requests.Load() != 2 {
		t.Fatalf("tool failure event=%v requests=%d", failed, requests.Load())
	}
}

func TestAgentBoundsActualToolExecutions(t *testing.T) {
	t.Parallel()
	search := &agentSearchFake{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		startAgentReply(w, 10)
		agentToolCall(w)
		finishAgentReply(w, "tool_use", 5)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	reader, err := NewChatModel().OpenStream(ctx, aiapp.TextInvocation{Runtime: aiapp.RuntimeConfig{ChatModelBaseURL: server.URL, ChatModelAPIKey: "test-key"}, Model: "test-model", Messages: []*schema.Message{schema.UserMessage("找教程")}, Tools: []tool.BaseTool{search}})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var terminal error
	for {
		_, err := reader.Recv()
		if err != nil {
			terminal = err
			break
		}
	}
	if errors.Is(terminal, io.EOF) || search.calls.Load() != 2 {
		t.Fatalf("terminal=%v searches=%d", terminal, search.calls.Load())
	}
}

func sendAgentEvent(w http.ResponseWriter, name string, value any) {
	raw, _ := json.Marshal(value)
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, raw)
}

func startAgentReply(w http.ResponseWriter, inputTokens int) {
	w.Header().Set("Content-Type", "text/event-stream")
	sendAgentEvent(w, "message_start", map[string]any{"type": "message_start", "message": map[string]any{"id": "msg_agent", "type": "message", "role": "assistant", "model": "test-model", "content": []any{}, "usage": map[string]int{"input_tokens": inputTokens, "output_tokens": 0}}})
}

func agentText(w http.ResponseWriter, text string) {
	sendAgentEvent(w, "content_block_start", map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "text", "text": ""}})
	sendAgentEvent(w, "content_block_delta", map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": text}})
	sendAgentEvent(w, "content_block_stop", map[string]any{"type": "content_block_stop", "index": 0})
}

func agentToolCall(w http.ResponseWriter) {
	sendAgentEvent(w, "content_block_start", map[string]any{"type": "content_block_start", "index": 1, "content_block": map[string]any{"type": "tool_use", "id": "toolu_search_1", "name": aiapp.VideoSearchToolName, "input": map[string]any{}}})
	for _, part := range []string{`{"query":"Go`, ` 教程"}`} {
		sendAgentEvent(w, "content_block_delta", map[string]any{"type": "content_block_delta", "index": 1, "delta": map[string]any{"type": "input_json_delta", "partial_json": part}})
	}
	sendAgentEvent(w, "content_block_stop", map[string]any{"type": "content_block_stop", "index": 1})
}

func finishAgentReply(w http.ResponseWriter, reason string, outputTokens int) {
	sendAgentEvent(w, "message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": reason}, "usage": map[string]int{"output_tokens": outputTokens}})
	sendAgentEvent(w, "message_stop", map[string]any{"type": "message_stop"})
}

func TestAgentOrdinaryChatUsesOneRequestWithoutExecutingTools(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	search := &agentSearchFake{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var body struct {
			Tools  []struct{ Name string }
			Stream bool
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if len(body.Tools) != 1 || body.Tools[0].Name != aiapp.VideoSearchToolName || !body.Stream {
			t.Errorf("tool schema not sent: %+v", body)
		}
		startAgentReply(w, 10)
		agentText(w, "你好，可以直接聊天。")
		finishAgentReply(w, "end_turn", 4)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	reader, err := NewChatModel().OpenStream(ctx, aiapp.TextInvocation{Runtime: aiapp.RuntimeConfig{ChatModelBaseURL: server.URL, ChatModelAPIKey: "test-key"}, Model: "test-model", Messages: []*schema.Message{schema.UserMessage("你好")}, Tools: []tool.BaseTool{search}})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var text strings.Builder
	var meta aiapp.ResponseMetadata
	for {
		msg, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		text.WriteString(msg.Content)
		meta.Observe(msg)
		if _, ok := aiapp.MessageToolEvent(msg); ok {
			t.Fatal("ordinary chat emitted a tool event")
		}
	}
	if requests.Load() != 1 || search.calls.Load() != 0 || meta.Status() != "completed" || text.String() != "你好，可以直接聊天。" {
		t.Fatalf("requests=%d tools=%d text=%q meta=%+v", requests.Load(), search.calls.Load(), text.String(), meta)
	}
}

func TestAgentPreservesThinkingSignatureAcrossToolRound(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	search := &agentSearchFake{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			startAgentReply(w, 10)
			sendAgentEvent(w, "content_block_start", map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "thinking", "thinking": ""}})
			sendAgentEvent(w, "content_block_delta", map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "thinking_delta", "thinking": "需要搜索站内视频"}})
			sendAgentEvent(w, "content_block_delta", map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "signature_delta", "signature": "signature_for_test"}})
			sendAgentEvent(w, "content_block_stop", map[string]any{"type": "content_block_stop", "index": 0})
			agentToolCall(w)
			finishAgentReply(w, "tool_use", 5)
		} else {
			body, _ := io.ReadAll(r.Body)
			for _, required := range []string{`"type":"thinking"`, `"signature":"signature_for_test"`, `"tool_use_id":"toolu_search_1"`} {
				if !strings.Contains(string(body), required) {
					t.Errorf("continuation lost %s: %s", required, body)
				}
			}
			startAgentReply(w, 20)
			agentText(w, "找到教程。")
			finishAgentReply(w, "end_turn", 4)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	thinking := true
	reader, err := NewChatModel().OpenStream(ctx, aiapp.TextInvocation{
		Runtime: aiapp.RuntimeConfig{ChatModelBaseURL: server.URL, ChatModelAPIKey: "test-key"}, Model: "test-model",
		Messages: []*schema.Message{schema.UserMessage("找教程")}, Tools: []tool.BaseTool{search}, Thinking: &thinking,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	for {
		_, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if requests.Load() != 2 || search.calls.Load() != 1 {
		t.Fatalf("requests=%d calls=%d", requests.Load(), search.calls.Load())
	}
}

func TestAgentStreamsTextThenExecutesSearchAndReturnsResult(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	search := &agentSearchFake{}
	prefixSeen := make(chan struct{})
	var prefixOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		round := requests.Add(1)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if round == 1 {
			startAgentReply(w, 10)
			agentText(w, "我来搜索。")
			w.(http.Flusher).Flush()
			select {
			case <-prefixSeen:
			case <-r.Context().Done():
				return
			case <-time.After(3 * time.Second):
				t.Error("intermediate text was buffered until the tool round finished")
				return
			}
			agentToolCall(w)
			finishAgentReply(w, "tool_use", 5)
		} else {
			raw, _ := json.Marshal(body["messages"])
			if !strings.Contains(string(raw), `"tool_use_id":"toolu_search_1"`) || !strings.Contains(string(raw), "/video/7") {
				t.Errorf("tool result not returned to model: %s", raw)
			}
			startAgentReply(w, 20)
			agentText(w, "找到教程了。")
			finishAgentReply(w, "end_turn", 4)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	reader, err := NewChatModel().OpenStream(ctx, aiapp.TextInvocation{Runtime: aiapp.RuntimeConfig{ChatModelBaseURL: server.URL, ChatModelAPIKey: "test-key"}, Model: "test-model", Messages: []*schema.Message{schema.UserMessage("找 Go 教程")}, Tools: []tool.BaseTool{search}})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var text strings.Builder
	var meta aiapp.ResponseMetadata
	var events []aiapp.ToolCallEvent
	for {
		msg, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		text.WriteString(msg.Content)
		meta.Observe(msg)
		if strings.Contains(text.String(), "我来搜索。") {
			prefixOnce.Do(func() { close(prefixSeen) })
		}
		if event, ok := aiapp.MessageToolEvent(msg); ok {
			events = append(events, event)
		}
	}
	if requests.Load() != 2 || search.calls.Load() != 1 || !strings.Contains(text.String(), "找到教程了。") {
		t.Fatalf("requests=%d tools=%d text=%q", requests.Load(), search.calls.Load(), text.String())
	}
	if len(events) != 2 || events[0].Phase != "started" || events[1].Phase != "completed" || events[0].CallID != "toolu_search_1" || events[1].ResultCount != 2 || events[0].Query != "Go 教程" {
		t.Fatalf("tool events=%+v", events)
	}
	if meta.Status() != "completed" || meta.Usage == nil || meta.Usage.InputTokens != 30 || meta.Usage.OutputTokens != 9 {
		t.Fatalf("aggregate metadata=%+v usage=%+v", meta, meta.Usage)
	}
}

func TestAgentNonStreamingDoesNotExecuteIncompleteToolCall(t *testing.T) {
	t.Parallel()
	for _, reason := range []string{"max_tokens", ""} {
		t.Run(reason, func(t *testing.T) {
			t.Parallel()
			search := &agentSearchFake{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id": "msg_incomplete_tool", "type": "message", "role": "assistant", "model": "test-model",
					"content":     []any{map[string]any{"type": "tool_use", "id": "toolu_search_1", "name": aiapp.VideoSearchToolName, "input": map[string]string{"query": "Go 教程"}}},
					"stop_reason": reason, "usage": map[string]int{"input_tokens": 10, "output_tokens": 5},
				})
			}))
			defer server.Close()
			result, err := NewChatModel().InvokeText(t.Context(), aiapp.TextInvocation{Runtime: aiapp.RuntimeConfig{ChatModelBaseURL: server.URL, ChatModelAPIKey: "test-key"}, Model: "test-model", Messages: []*schema.Message{schema.UserMessage("找视频")}, Tools: []tool.BaseTool{search}})
			if err != nil || search.calls.Load() != 0 || result.Status() != "incomplete" || result.FinishReason != reason || result.Usage == nil || result.Usage.TotalTokens != 15 {
				t.Fatalf("error=%v executed=%d result=%+v usage=%+v", err, search.calls.Load(), result, result.Usage)
			}
		})
	}
}

func TestAgentDoesNotExecuteTruncatedOrUnterminatedToolCall(t *testing.T) {
	t.Parallel()
	for _, reason := range []string{"max_tokens", ""} {
		t.Run(reason, func(t *testing.T) {
			t.Parallel()
			search := &agentSearchFake{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				startAgentReply(w, 10)
				agentToolCall(w)
				if reason != "" {
					finishAgentReply(w, reason, 5)
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			reader, err := NewChatModel().OpenStream(ctx, aiapp.TextInvocation{Runtime: aiapp.RuntimeConfig{ChatModelBaseURL: server.URL, ChatModelAPIKey: "test-key"}, Model: "test-model", Messages: []*schema.Message{schema.UserMessage("找视频")}, Tools: []tool.BaseTool{search}})
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			var receivedErr error
			var metadata aiapp.ResponseMetadata
			for {
				msg, err := reader.Recv()
				metadata.Observe(msg)
				if err != nil {
					receivedErr = err
					break
				}
			}
			if !errors.Is(receivedErr, io.EOF) || search.calls.Load() != 0 || metadata.Status() != "incomplete" || metadata.FinishReason != reason {
				t.Fatalf("error=%v executed=%d metadata=%+v", receivedErr, search.calls.Load(), metadata)
			}
		})
	}
}
