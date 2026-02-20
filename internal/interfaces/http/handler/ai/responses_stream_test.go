package ai

import (
	"bytes"
	"testing"

	"github.com/cloudwego/eino/schema"
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
