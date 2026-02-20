package ai

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"

	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ResponsesView 聊天生成（POST，请求体为 ResponsesRequest；响应为 SSE 流）.
func (h *Handler) ResponsesView(c *gin.Context) {
	req := bound[aiapp.ResponsesRequest](c)
	h.streamResponses(c, req)
}

// ResponsesStreamReq 流式聊天的 query 参数.
type ResponsesStreamReq struct {
	Token          string `form:"token" binding:"omitempty" label:"访问令牌"`
	Model          string `form:"model" binding:"omitempty" label:"模型"`
	Text           string `form:"text" binding:"omitempty" label:"用户文本"`
	Input          string `form:"input" binding:"omitempty" label:"原始 input JSON"`
	Thinking       *bool  `form:"thinking" binding:"omitempty" label:"是否思考"`
	ThinkingEffort string `form:"thinkingEffort" binding:"omitempty" label:"思考强度"`
}

// ResponsesStreamView 流式聊天（GET + query 参数，便于 EventSource 直连）.
func (h *Handler) ResponsesStreamView(c *gin.Context) {
	req := bound[ResponsesStreamReq](c)

	streamReq, err := buildResponsesStreamRequest(req)
	if err != nil {
		h.failWithAIError(c, "build AI response stream request failed", err)

		return
	}

	h.streamResponses(c, streamReq)
}

// buildResponsesStreamRequest 把 query 参数转成聊天请求：优先 input，其次 text。
func buildResponsesStreamRequest(req ResponsesStreamReq) (aiapp.ResponsesRequest, error) {
	if input := strings.TrimSpace(req.Input); input != "" {
		raw := json.RawMessage(input)
		if !json.Valid(raw) {
			return aiapp.ResponsesRequest{}, errors.New("input 不是合法 JSON")
		}

		return aiapp.ResponsesRequest{
			Model:          req.Model,
			Input:          raw,
			Thinking:       req.Thinking,
			ThinkingEffort: req.ThinkingEffort,
		}, nil
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		return aiapp.ResponsesRequest{}, errors.New("text 不能为空")
	}

	raw, err := json.Marshal([]map[string]any{
		{
			"role": "user",
			"content": []map[string]any{
				{
					"type": "input_text",
					"text": text,
				},
			},
		},
	})
	if err != nil {
		return aiapp.ResponsesRequest{}, err
	}

	return aiapp.ResponsesRequest{
		Model:          req.Model,
		Input:          raw,
		Thinking:       req.Thinking,
		ThinkingEffort: req.ThinkingEffort,
	}, nil
}

// streamResponses 打开上游流并逐块转发给客户端（禁用代理缓冲，逐块 Flush）。
func (h *Handler) streamResponses(c *gin.Context, req aiapp.ResponsesRequest) {
	stream, err := h.service.OpenResponseStream(c.Request.Context(), req)
	if err != nil {
		h.failWithAIError(c, "open AI response stream failed", err)

		return
	}
	defer stream.Close()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	// Disable nginx/proxy buffering
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	c.Status(200)

	for {
		chunk, receiveErr := stream.Recv()
		if errors.Is(receiveErr, io.EOF) {
			if err = writeResponsesCompleted(c.Writer); err != nil {
				h.log().Warn("write AI response completion failed", zap.Error(err))

				return
			}
			c.Writer.Flush()

			return
		}
		if receiveErr != nil {
			h.log().Warn("receive Eino AI response stream failed", zap.Error(receiveErr))

			return
		}
		if err = writeResponsesChunk(c.Writer, chunk); err != nil {
			h.log().Warn("write AI response stream failed", zap.Error(err))

			return
		}
		c.Writer.Flush()
	}
}

func writeResponsesChunk(writer io.Writer, chunk *schema.Message) error {
	if chunk == nil {
		return nil
	}

	if chunk.ReasoningContent != "" {
		if err := writeResponsesEvent(writer, map[string]any{
			"type":  "response.reasoning_summary_text.delta",
			"delta": chunk.ReasoningContent,
		}); err != nil {
			return err
		}
	}
	if chunk.Content == "" {
		return nil
	}

	return writeResponsesEvent(writer, map[string]any{
		"type":  "response.output_text.delta",
		"delta": chunk.Content,
	})
}

func writeResponsesCompleted(writer io.Writer) error {
	if err := writeResponsesEvent(writer, map[string]any{"type": "response.completed"}); err != nil {
		return err
	}

	_, err := io.WriteString(writer, "data: [DONE]\n\n")

	return err
}

func writeResponsesEvent(writer io.Writer, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err = io.WriteString(writer, "data: "); err != nil {
		return err
	}
	if _, err = writer.Write(raw); err != nil {
		return err
	}
	_, err = io.WriteString(writer, "\n\n")

	return err
}
