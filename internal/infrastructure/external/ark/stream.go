package ark

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

const (
	// streamScannerInitial SSE 扫描器初始缓冲。
	streamScannerInitial = 64 * 1024
	// streamScannerMax SSE 单行上限（思考增量可能很长）。
	streamScannerMax = 2 * 1024 * 1024
)

// translateAnthropicStream 把上游 Anthropic SSE 流翻译为前端使用的 Responses SSE 流。
//
// 输出事件格式（response.output_text.delta / response.reasoning_summary_text.delta /
// response.completed / [DONE]）属于外部契约，逐字保持不变。
func translateAnthropicStream(upstream *http.Response) *http.Response {
	reader, writer := io.Pipe()
	header := http.Header{}
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")

	go func() {
		defer func() { _ = upstream.Body.Close() }()
		defer func() { _ = writer.Close() }()

		scanner := bufio.NewScanner(upstream.Body)
		scanner.Buffer(make([]byte, 0, streamScannerInitial), streamScannerMax)

		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "event:") {
				continue
			}

			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" {
				continue
			}

			if data == "[DONE]" {
				_, _ = writer.Write([]byte("data: [DONE]\n\n"))

				continue
			}

			event, ok := translateAnthropicDataEvent(data)
			if !ok {
				continue
			}

			if _, err := writer.Write(event); err != nil {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			_ = writer.CloseWithError(err)
		}
	}()

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     header,
		Body:       reader,
	}
}

// translateAnthropicDataEvent 翻译单条 SSE data 事件；返回 false 表示忽略该事件。
func translateAnthropicDataEvent(data string) ([]byte, bool) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return nil, false
	}

	eventType, _ := payload[jsonKeyType].(string)

	switch eventType {
	case "content_block_delta":
		delta, _ := payload["delta"].(map[string]any)
		if delta == nil {
			return nil, false
		}

		deltaType, _ := delta[jsonKeyType].(string)

		switch strings.TrimSpace(deltaType) {
		case "thinking_delta":
			// Anthropic / DeepSeek 思考增量
			thinking, _ := delta["thinking"].(string)
			if thinking == "" {
				return nil, false
			}

			return marshalSSE(map[string]any{
				jsonKeyType: "response.reasoning_summary_text.delta",
				"delta":     thinking,
			}), true
		case "text_delta", "":
			text, _ := delta["text"].(string)
			if text == "" {
				return nil, false
			}

			return marshalSSE(map[string]any{
				jsonKeyType: "response.output_text.delta",
				"delta":     text,
			}), true
		default:
			// 兼容未带 type、仅有 text 的上游
			if text, _ := delta["text"].(string); text != "" {
				return marshalSSE(map[string]any{
					jsonKeyType: "response.output_text.delta",
					"delta":     text,
				}), true
			}

			return nil, false
		}
	case "message_stop":
		return []byte("data: {\"type\":\"response.completed\"}\n\ndata: [DONE]\n\n"), true
	case "response.output_text.delta", "response.reasoning_summary_text.delta", "response.completed":
		return []byte("data: " + data + "\n\n"), true
	default:
		return nil, false
	}
}

// marshalSSE 序列化为一条 SSE data 帧。
func marshalSSE(payload any) []byte {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil
	}

	return append(append([]byte("data: "), raw...), []byte("\n\n")...)
}
