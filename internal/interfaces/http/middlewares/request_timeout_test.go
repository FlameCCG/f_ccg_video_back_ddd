package middlewares

import (
	"context"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRequestTimeoutAddsDeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.GET("/ping", RequestTimeout(ctxutil.RequestFastTimeout), func(c *gin.Context) {
		deadline, ok := c.Request.Context().Deadline()
		if !ok {
			t.Fatal("expected request ctx deadline")
		}

		if remaining := time.Until(deadline); remaining <= 0 || remaining > ctxutil.RequestFastTimeout {
			t.Fatalf("unexpected remaining timeout: %v", remaining)
		}

		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}
}
