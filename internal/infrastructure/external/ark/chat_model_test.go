package ark

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"

	"github.com/cloudwego/eino/schema"
)

const (
	testTestKey = "test-key"
)

type capturedChatRequest struct {
	path   string
	apiKey string
	body   map[string]any
	err    error
}

func TestInvokeTextUsesEinoClaudeModel(t *testing.T) {
	t.Parallel()

	requests := make(chan capturedChatRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		raw, err := io.ReadAll(request.Body)

		captured := capturedChatRequest{
			path:   request.URL.Path,
			apiKey: request.Header.Get("x-api-key"),
			err:    err,
		}
		if err == nil {
			captured.err = json.Unmarshal(raw, &captured.body)
		}

		requests <- captured

		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{
			"id":"msg_test","type":"message","role":"assistant","model":"deepseek-v4-flash",
			"content":[{"type":"text","text":"Eino 已接管"}],"stop_reason":"end_turn",
			"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":4}
		}`)
	}))
	t.Cleanup(server.Close)

	runtime := aiapp.RuntimeConfig{
		ChatModelBaseURL: server.URL,
		ChatModelAPIKey:  testTestKey,
	}
	model := NewChatModel()
	thinking := true

	text, err := model.InvokeText(context.Background(), aiapp.TextInvocation{
		Runtime: runtime,
		Model:   "deepseek-v4-flash",
		Messages: []*schema.Message{
			schema.SystemMessage("系统提示"),
			schema.UserMessage("你好"),
		},
		Thinking:       &thinking,
		ThinkingEffort: "max",
		MaxTokens:      defaultChatMaxTokens,
	})
	require.NoError(t, err, "InvokeText() error = %v", err)

	require.Equal(t, "Eino 已接管", text, "InvokeText() = %q", text)

	captured := <-requests
	require.NoError(t, captured.err, "capture request error = %v", captured.err)

	require.Equal(t, "/v1/messages", captured.path, "path=%q apiKey=%q", captured.path, captured.apiKey)
	require.Equal(t, testTestKey, captured.apiKey, "path=%q apiKey=%q", captured.path, captured.apiKey)

	thinkingPayload := requireMapField(t, captured.body, "thinking")
	require.EqualValues(t, "enabled", thinkingPayload["type"], "thinking.type = %#v", thinkingPayload["type"])

	outputConfig := requireMapField(t, captured.body, "output_config")
	require.EqualValues(t, "max", outputConfig["effort"], "output_config.effort = %#v", outputConfig["effort"])

	{
		_, ok := captured.body["system"]
		require.True(t, ok, "request body missing system")
	}

	{
		messages, ok := captured.body["messages"].([]any)
		require.True(t, ok, "messages = %#v", captured.body["messages"])
		require.Len(t, messages, 1, "messages = %#v", captured.body["messages"])
	}
}

func TestOpenStreamUsesEinoClaudeModel(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(writer, strings.Join([]string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_stream","type":"message","role":"assistant","model":"deepseek-v4-flash","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":2,"output_tokens":0}}}`,
			"",
			`event: content_block_start`,
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			"",
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Eino 流式输出"}}`,
			"",
			`event: content_block_stop`,
			`data: {"type":"content_block_stop","index":0}`,
			"",
			`event: message_delta`,
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":4}}`,
			"",
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			"",
			"",
		}, "\n"))
	}))
	t.Cleanup(server.Close)

	runtime := aiapp.RuntimeConfig{
		ChatModelBaseURL: server.URL,
		ChatModelAPIKey:  testTestKey,
	}
	model := NewChatModel()

	stream, err := model.OpenStream(context.Background(), aiapp.TextInvocation{
		Runtime:   runtime,
		Model:     "deepseek-v4-flash",
		Messages:  []*schema.Message{schema.UserMessage("请流式回答")},
		MaxTokens: defaultChatMaxTokens,
	})
	require.NoError(t, err, "OpenStream() error = %v", err)

	defer stream.Close()

	var content strings.Builder

	for {
		chunk, receiveErr := stream.Recv()
		if errors.Is(receiveErr, io.EOF) {
			break
		}

		require.NoError(t, receiveErr, "stream.Recv() error = %v", receiveErr)

		if chunk != nil {
			content.WriteString(chunk.Content)
		}
	}

	{
		got := content.String()
		require.Equal(t, "Eino 流式输出", got, "stream content = %q", got)
	}
}

func requireMapField(t *testing.T, payload map[string]any, field string) map[string]any {
	t.Helper()

	value, ok := payload[field].(map[string]any)
	require.True(t, ok, "%s = %#v, want object", field, payload[field])

	return value
}

func TestOpenStreamReturnsProviderErrorWithoutWaitingForCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid API key"}}`)
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stream, err := NewChatModel().OpenStream(ctx, aiapp.TextInvocation{
		Runtime: aiapp.RuntimeConfig{ChatModelBaseURL: server.URL, ChatModelAPIKey: testTestKey},
		Model:   "test-model", Messages: []*schema.Message{schema.UserMessage("你好")},
	})
	if stream != nil {
		stream.Close()
	}

	require.NoError(t, ctx.Err(), "OpenStream waited for cancellation instead of returning provider error: %v", err)

	require.Error(t, err, "OpenStream() error = %v, want provider authentication error", err)
	require.Contains(t, err.Error(), "401", "OpenStream() error = %v, want provider authentication error", err)
}
