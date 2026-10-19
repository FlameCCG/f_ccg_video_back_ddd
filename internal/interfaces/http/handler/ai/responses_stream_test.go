package ai

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
)

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

func TestForwardResponsesStream(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		streamErr error
		cancelled bool
		want      string
		absent    string
	}{
		{name: "completed", want: "response.completed", absent: "response.failed"},
		{name: "upstream failed", streamErr: errors.New("private upstream credentials"), want: "response.failed", absent: "response.completed"},
		{name: "cancelled", cancelled: true, absent: "data:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stream, writer := schema.Pipe[*schema.Message](2)
			defer stream.Close()
			writer.Send(&schema.Message{Content: "部分回答"}, nil)
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
	if err := writeResponsesCompleted(&output); err != nil {
		t.Fatalf("writeResponsesCompleted() error = %v", err)
	}

	const expected = "data: {\"type\":\"response.completed\"}\n\ndata: [DONE]\n\n"
	if output.String() != expected {
		t.Fatalf("writeResponsesCompleted() = %q, want %q", output.String(), expected)
	}
}
