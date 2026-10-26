package middlewares

import (
	"context"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestTimeoutAddsDeadline(t *testing.T) {
	t.Parallel()

	engine := gin.New()
	engine.GET("/ping", RequestTimeout(ctxutil.RequestFastTimeout), func(c *gin.Context) {
		deadline, ok := c.Request.Context().Deadline()
		require.True(t, ok, "expected request ctx deadline")

		{
			remaining := time.Until(deadline)
			require.Positive(t, remaining, "unexpected remaining timeout: %v", remaining)
			require.LessOrEqual(t, remaining, ctxutil.RequestFastTimeout, "unexpected remaining timeout: %v", remaining)
		}

		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	engine.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusNoContent, recorder.Code, "unexpected status code: %d", recorder.Code)
}
