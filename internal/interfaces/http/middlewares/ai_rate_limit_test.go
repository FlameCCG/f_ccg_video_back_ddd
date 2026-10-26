package middlewares

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAIBudgetsUserIPAndDailyQuota(t *testing.T) {
	t.Parallel()

	server, limiter := newLoginRateLimiter(t)
	mw := &Middlewares{limiter: limiter}
	engine := gin.New()
	engine.POST("/ai", func(c *gin.Context) { c.Set("uid", c.GetHeader("X-Test-UID")); c.Next() }, mw.AIGenerationLimit(), func(c *gin.Context) { c.Status(204) })

	send := func(uid int, ip string) int {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/ai", nil)
		r.RemoteAddr = ip + ":1234"
		r.Header.Set("X-Test-UID", strconv.Itoa(uid))

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)

		return w.Code
	}

	for range 6 {
		{
			code := send(1, "192.0.2.1")
			require.Equal(t, 204, code, code)
		}
	}

	{
		code := send(1, "192.0.2.2")
		require.Equal(t, 429, code, "user burst bypass=%d", code)
	}

	for uid := 2; uid <= 5; uid++ {
		for range 6 {
			send(uid, "192.0.2.9")
		}
	}

	{
		code := send(6, "192.0.2.9")
		require.Equal(t, 429, code, "IP burst bypass=%d", code)
	}

	for range 9 {
		server.FastForward(time.Minute)

		for range 6 {
			send(10, "192.0.2.10")
		}
	}

	server.FastForward(time.Minute)

	{
		code := send(10, "192.0.2.10")
		require.Equal(t, 429, code, "daily quota bypass=%d", code)
	}

	server.Close()

	{
		code := send(99, "192.0.2.99")
		require.NotEqual(t, 204, code, "Redis failure permitted paid request")
	}
}
