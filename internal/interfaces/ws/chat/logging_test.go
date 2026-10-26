package chat

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/internal/interfaces/ws/wsorigin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestWebSocketUpgradeStatusAndBoundaryLog(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	hub := NewHub(nil, nil, wsorigin.NewChecker(nil), logger)
	engine := gin.New()
	engine.Use(middlewares.RequestContext(), middlewares.GinLogger(logger), middlewares.GinRecovery(logger, true))

	finished := make(chan struct{})

	engine.GET("/ws", func(c *gin.Context) { defer close(finished); _ = hub.ServeWS(c, 0) })

	server := httptest.NewServer(engine)
	defer server.Close()

	conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws?token=private-token", http.Header{"Origin": []string{server.URL}})
	require.NoError(t, err)

	if response.Body != nil {
		defer func() { require.NoError(t, response.Body.Close()) }()
	}

	require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode, "bad handshake: %+v", response)
	require.NotEmpty(t, response.Header.Get("X-Request-ID"), "bad handshake: %+v", response)

	_ = conn.Close()

	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("WebSocket reader did not stop")
	}
	// The request middleware logs after the handler returns.
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()

	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()

	for logs.FilterMessage("http.request").Len() == 0 {
		select {
		case <-deadline.C:
			t.Fatal("missing upgrade summary")
		case <-tick.C:
		}
	}

	summary := logs.FilterMessage("http.request").All()[0].ContextMap()
	require.EqualValues(t, int64(101), summary["http_status"], "bad upgrade summary: %#v", summary)
	require.EqualValues(t, true, summary["success"], "bad upgrade summary: %#v", summary)
}
