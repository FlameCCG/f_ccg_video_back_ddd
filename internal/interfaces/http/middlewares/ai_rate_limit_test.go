package middlewares

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestAIBudgetsUserIPAndDailyQuota(t *testing.T) {
	server, limiter := newLoginRateLimiter(t)
	mw := &Middlewares{limiter: limiter}
	engine := gin.New()
	engine.POST("/ai", func(c *gin.Context) { c.Set("uid", c.GetHeader("X-Test-UID")); c.Next() }, mw.AIGenerationLimit(), func(c *gin.Context) { c.Status(204) })
	send := func(uid int, ip string) int {
		r := httptest.NewRequest(http.MethodPost, "/ai", nil)
		r.RemoteAddr = ip + ":1234"
		r.Header.Set("X-Test-UID", strconv.Itoa(uid))
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		return w.Code
	}
	for i := 0; i < 6; i++ {
		if code := send(1, "192.0.2.1"); code != 204 {
			t.Fatal(code)
		}
	}
	if code := send(1, "192.0.2.2"); code != 429 {
		t.Fatalf("user burst bypass=%d", code)
	}
	for uid := 2; uid <= 5; uid++ {
		for i := 0; i < 6; i++ {
			send(uid, "192.0.2.9")
		}
	}
	if code := send(6, "192.0.2.9"); code != 429 {
		t.Fatalf("IP burst bypass=%d", code)
	}
	for batch := 0; batch < 9; batch++ {
		server.FastForward(time.Minute)
		for i := 0; i < 6; i++ {
			send(10, "192.0.2.10")
		}
	}
	server.FastForward(time.Minute)
	if code := send(10, "192.0.2.10"); code != 429 {
		t.Fatalf("daily quota bypass=%d", code)
	}
	server.Close()
	if code := send(99, "192.0.2.99"); code == 204 {
		t.Fatal("Redis failure permitted paid request")
	}
}
