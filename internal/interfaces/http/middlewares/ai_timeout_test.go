package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"f_ccg_video_back_ddd/internal/config"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/gin-gonic/gin"
)

func TestAIRequestTimeoutTracksReloadAndPreservesParentDeadline(t *testing.T) {
	t.Parallel()
	provider := infraconfig.NewProvider(&config.Config{})
	m := &Middlewares{cfgProvider: provider}
	engine := gin.New()
	engine.Use(m.AIRequestTimeout())
	var received time.Time
	engine.GET("/ai", func(c *gin.Context) {
		received, _ = c.Request.Context().Deadline()
		c.Status(http.StatusNoContent)
	})
	for _, seconds := range []int{0, 5, 180} {
		provider.Set(&config.Config{AI: &config.AI{TimeoutSec: seconds}})
		engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ai", nil))
		want := time.Duration(seconds) * time.Second
		if seconds == 0 {
			want = 90 * time.Second
		}
		if remaining := time.Until(received); remaining > want || remaining < want-time.Second {
			t.Fatalf("configured=%d remaining=%s", seconds, remaining)
		}
	}
	parent, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ai", nil).WithContext(parent))
	want, _ := parent.Deadline()
	if !received.Equal(want) {
		t.Fatalf("parent deadline extended: got=%v want=%v", received, want)
	}
}
