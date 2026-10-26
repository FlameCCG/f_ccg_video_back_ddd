package ai

import (
	"bytes"
	"context"
	"errors"
	"f_ccg_video_back_ddd/pkg/res"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWriteResponsesChunk(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer

	err := writeResponsesChunk(&output, &schema.Message{
		Role:             schema.Assistant,
		ReasoningContent: "先分析",
		Content:          "最终回答",
	})
	require.NoError(t, err, "writeResponsesChunk() error = %v", err)

	const expected = "data: {\"delta\":\"先分析\",\"type\":\"response.reasoning_summary_text.delta\"}\n\n" +
		"data: {\"delta\":\"最终回答\",\"type\":\"response.output_text.delta\"}\n\n"
	require.Equal(t, expected, output.String(), "writeResponsesChunk() = %q, want %q", output.String(), expected)
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

			c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/responses", nil).WithContext(ctx)
			NewHandler(nil, nil).forwardResponsesStream(c, stream)

			success, known := res.Success(c)
			require.True(t, known, "SSE result known=%v success=%v", known, success)
			require.Equal(t, success, (tc.streamErr == nil && !tc.cancelled), "SSE result known=%v success=%v", known, success)

			require.False(t, tc.streamErr != nil && res.Failure(c) == nil, "SSE error cause lost")

			body := recorder.Body.String()
			require.False(t, tc.want != "" && !strings.Contains(body, tc.want), "response = %q, want %q", body, tc.want)

			require.NotContains(t, body, tc.absent, "response contains forbidden event or private error: %q", body)
			require.NotContains(t, body, "private upstream credentials", "response contains forbidden event or private error: %q", body)
		})
	}
}

func TestWriteResponsesCompleted(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	{
		err := writeResponsesCompleted(&output)
		require.NoError(t, err, "writeResponsesCompleted() error = %v", err)
	}

	const expected = "data: {\"type\":\"response.completed\"}\n\ndata: [DONE]\n\n"
	require.Equal(t, expected, output.String(), "writeResponsesCompleted() = %q, want %q", output.String(), expected)
}
