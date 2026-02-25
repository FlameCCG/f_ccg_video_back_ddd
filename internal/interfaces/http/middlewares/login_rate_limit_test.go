package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"f_ccg_video_back_ddd/internal/infrastructure/cache/ratelimit"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// newLoginRateTestEngine 装配仅带 LoginRateLimit 的最小链路，落点处理器回显读到的 body。
func newLoginRateTestEngine(t *testing.T, limiter *ratelimit.Limiter) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)

	mw := New(nil, nil, nil, nil, nil, nil, limiter, zap.NewNop())

	engine := gin.New()

	engine.POST("/login", mw.LoginRateLimit(), func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, "ok:"+string(body))
	})

	return engine
}

func newLoginRateLimiter(t *testing.T) (*miniredis.Miniredis, *ratelimit.Limiter) {
	t.Helper()

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})

	t.Cleanup(func() { _ = client.Close() })

	return server, ratelimit.New(client)
}

func postLogin(t *testing.T, engine *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()

	return postLoginFromIP(t, engine, body, "192.0.2.10:56789")
}

// postLoginFromIP 以指定来源地址发起登录请求（gin 在无 XFF 头时取 RemoteAddr 作为 ClientIP）。
func postLoginFromIP(t *testing.T, engine *gin.Engine, body, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	return recorder
}

// TestLoginRateLimitAccountDimension 账号维度：同 IP 同 username 超过阈值后拒绝，
// 且通过的请求 body 被完整复位供后续中间件读取。
func TestLoginRateLimitAccountDimension(t *testing.T) {
	t.Parallel()

	_, limiter := newLoginRateLimiter(t)
	engine := newLoginRateTestEngine(t, limiter)

	body := `{"username":"Tester@Example.com","password":"wrong"}`

	for i := 1; i <= int(loginRateAccountLimit); i++ {
		rec := postLogin(t, engine, body)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ok:"+body) {
			t.Fatalf("request %d should pass with body preserved, code=%d body=%q", i, rec.Code, rec.Body.String())
		}
	}

	rec := postLogin(t, engine, body)
	if !strings.Contains(rec.Body.String(), "登录尝试过于频繁") {
		t.Fatalf("request beyond account limit should be rejected, got code=%d body=%q", rec.Code, rec.Body.String())
	}
}

// TestLoginRateLimitAccountKeyedByIP 账号维度绑定 IP——任何知道 username 的人
// 只能烧掉自己 IP 上的额度：攻击者发满阈值后，受害者从其他网络登录不受影响。
func TestLoginRateLimitAccountKeyedByIP(t *testing.T) {
	t.Parallel()

	_, limiter := newLoginRateLimiter(t)
	engine := newLoginRateTestEngine(t, limiter)

	body := `{"username":"victim@example.com","password":"wrong"}`

	// 攻击者（1.1.1.1）用受害者 username 发满「IP + 账号」阈值
	for i := 1; i <= int(loginRateAccountLimit); i++ {
		rec := postLoginFromIP(t, engine, body, "1.1.1.1:1111")
		if rec.Code != http.StatusOK {
			t.Fatalf("attacker request %d should pass, code=%d body=%q", i, rec.Code, rec.Body.String())
		}
	}

	// 攻击者继续尝试：被拒
	if rec := postLoginFromIP(t, engine, body, "1.1.1.1:1111"); !strings.Contains(rec.Body.String(), "登录尝试过于频繁") {
		t.Fatalf("attacker beyond limit should be rejected, got code=%d body=%q", rec.Code, rec.Body.String())
	}

	// 受害者（2.2.2.2）同一账号正常登录：不受攻击者影响
	if rec := postLoginFromIP(t, engine, body, "2.2.2.2:2222"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ok:") {
		t.Fatalf("victim from another IP should pass, got code=%d body=%q", rec.Code, rec.Body.String())
	}
}

// TestLoginRateLimitIPDimension IP 维度：每次换不同账号（或无 username 的 OAuth 体）
// 只按 IP 计数，超过阈值后仍被拒绝。
func TestLoginRateLimitIPDimension(t *testing.T) {
	t.Parallel()

	_, limiter := newLoginRateLimiter(t)
	engine := newLoginRateTestEngine(t, limiter)

	for i := 1; i <= int(loginRateIPLimit); i++ {
		rec := postLogin(t, engine, `{"code":"anything"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d should pass, code=%d body=%q", i, rec.Code, rec.Body.String())
		}
	}

	rec := postLogin(t, engine, `{"username":"fresh_user","password":"x"}`)
	if !strings.Contains(rec.Body.String(), "登录尝试过于频繁") {
		t.Fatalf("request beyond IP limit should be rejected even with a fresh account, got code=%d body=%q", rec.Code, rec.Body.String())
	}
}

// TestLoginRateLimitFailsClosedOnRedisError Redis 异常时与邮箱验证码限流一致：
// 拒绝请求而不是放行，保证限流组件失效后不会退化为零防护。
func TestLoginRateLimitFailsClosedOnRedisError(t *testing.T) {
	t.Parallel()

	server, limiter := newLoginRateLimiter(t)
	server.Close()

	engine := newLoginRateTestEngine(t, limiter)

	rec := postLogin(t, engine, `{"username":"someone","password":"x"}`)
	if !strings.Contains(rec.Body.String(), "登录服务繁忙") {
		t.Fatalf("request should fail closed on redis error, got code=%d body=%q", rec.Code, rec.Body.String())
	}
}

// TestLoginRateLimitPassesWithoutLimiter 未装配 Limiter（如部分单测/降级部署）时放行。
func TestLoginRateLimitPassesWithoutLimiter(t *testing.T) {
	t.Parallel()

	engine := newLoginRateTestEngine(t, nil)

	rec := postLogin(t, engine, `{"username":"someone","password":"x"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ok:") {
		t.Fatalf("request should pass when limiter is nil, got code=%d body=%q", rec.Code, rec.Body.String())
	}
}
