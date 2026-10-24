package ipUtils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// stubRegionResolver 用固定返回值替代 ip2region，避免 pkg 层测试依赖基础设施实现。
// 真实归属地解析（含 xdb 查询与字段拼装）由
// internal/infrastructure/ipregion 的用例覆盖。
type stubRegionResolver struct {
	region string
	calls  []string
}

func (s *stubRegionResolver) Region(ip string) string {
	s.calls = append(s.calls, ip)

	return s.region
}

func TestGetIPAndAddrUsesClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	resolver := &stubRegionResolver{region: "广东·深圳"}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/common/comment/create", nil)
	ctx.Request.RemoteAddr = "8.8.8.8:12345"

	ip, addr := GetIPAndAddr(ctx, resolver)
	if ip != "8.8.8.8" {
		t.Fatalf("expected client ip, got %s", ip)
	}

	if addr == "" {
		t.Fatal("expected non-empty addr")
	}

	if len(resolver.calls) != 1 || resolver.calls[0] != "8.8.8.8" {
		t.Fatalf("resolver should be queried with the client ip, got %v", resolver.calls)
	}
}

func TestGetClientIPAndAddrPrefersXForwardedFor(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(recorder)
	_ = engine.SetTrustedProxies([]string{"127.0.0.0/8", "10.0.0.1"})
	engine.RemoteIPHeaders = []string{"X-Forwarded-For", "X-Real-IP"}

	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/common/user/login", nil)
	// 代理本机接入，真实客户端在转发头中
	ctx.Request.RemoteAddr = "127.0.0.1:54321"
	ctx.Request.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")
	ctx.Request.Header.Set("X-Real-IP", "203.0.113.10")

	ip, addr := GetClientIPAndAddr(ctx, &stubRegionResolver{region: "美国·加州"})
	if ip != "203.0.113.10" {
		t.Fatalf("expected forwarded client ip, got %s", ip)
	}

	if addr == "内网" {
		t.Fatalf("expected non-internal addr for public ip, got %s", addr)
	}
}

func TestGetClientIPAndAddrNormalizesMappedIPv6(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	ctx.Request.RemoteAddr = "127.0.0.1:1"
	ctx.Request.Header.Set("X-Real-IP", "::ffff:198.51.100.20")

	ip, _ := GetClientIPAndAddr(ctx, &stubRegionResolver{region: "美国"})
	if ip != "198.51.100.20" {
		t.Fatalf("expected normalized ipv4, got %s", ip)
	}
}

// TestGetClientIPAndAddrPrivateIPSkipsResolver 保证内网 IP 不查询归属地，
// 与原 GetIpAddr 先做 HasLocalIPAddr 判定的行为一致。
func TestGetClientIPAndAddrPrivateIPSkipsResolver(t *testing.T) {
	gin.SetMode(gin.TestMode)

	resolver := &stubRegionResolver{region: "不应被使用"}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	ctx.Request.RemoteAddr = "127.0.0.1:1"
	ctx.Request.Header.Set("X-Real-IP", "10.1.2.3")

	ip, addr := GetClientIPAndAddr(ctx, resolver)
	if ip != "10.1.2.3" {
		t.Fatalf("expected private ip, got %s", ip)
	}

	if addr != "内网" {
		t.Fatalf("expected 内网, got %s", addr)
	}

	if len(resolver.calls) != 0 {
		t.Fatalf("resolver should not be queried for private ip, got %v", resolver.calls)
	}
}

// TestGetClientIPAndAddrNilResolver 保证 resolver 缺失时公网 IP 返回「未知地址」，
// 与原实现中 global.IPSearcher 为 nil 的行为一致。
func TestGetClientIPAndAddrNilResolver(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	ctx.Request.RemoteAddr = "8.8.4.4:1"

	ip, addr := GetClientIPAndAddr(ctx, nil)
	if ip != "8.8.4.4" || addr != "未知地址" {
		t.Fatalf("got ip=%q addr=%q, want 8.8.4.4/未知地址", ip, addr)
	}
}

func TestNormalizeIPString(t *testing.T) {
	cases := map[string]string{
		" 8.8.4.4 ":         "8.8.4.4",
		"8.8.4.4:443":       "8.8.4.4",
		"[2001:db8::1]:443": "2001:db8::1",
		"::ffff:1.2.3.4":    "1.2.3.4",
		"not-an-ip":         "",
		"":                  "",
	}
	for in, want := range cases {
		if got := normalizeIPString(in); got != want {
			t.Fatalf("normalizeIPString(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestResolveClientIPRejectsSpoofedHeadersFromUntrustedPeer(t *testing.T) {
	ctx, engine := gin.CreateTestContext(httptest.NewRecorder())
	if err := engine.SetTrustedProxies([]string{"127.0.0.1"}); err != nil {
		t.Fatal(err)
	}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	ctx.Request.RemoteAddr = "198.51.100.10:1234"
	for _, header := range []string{"CF-Connecting-IP", "True-Client-IP", "X-Real-IP", "X-Forwarded-For"} {
		ctx.Request.Header.Set(header, "203.0.113.99")
	}
	if got := ResolveClientIP(ctx); got != "198.51.100.10" {
		t.Fatalf("spoofed header trusted: %s", got)
	}
}
