package ipUtils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
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
	t.Parallel()

	resolver := &stubRegionResolver{region: "广东·深圳"}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/common/comment/create", nil)
	ctx.Request.RemoteAddr = "8.8.8.8:12345"

	ip, addr := GetIPAndAddr(ctx, resolver)
	require.Equal(t, "8.8.8.8", ip, "expected client ip, got %s", ip)

	require.NotEmpty(t, addr, "expected non-empty addr")

	require.Len(t, resolver.calls, 1, "resolver should be queried with the client ip, got %v", resolver.calls)
	require.Equal(t, "8.8.8.8", resolver.calls[0], "resolver should be queried with the client ip, got %v", resolver.calls)
}

func TestGetClientIPAndAddrPrefersXForwardedFor(t *testing.T) {
	t.Parallel()

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
	require.Equal(t, "203.0.113.10", ip, "expected forwarded client ip, got %s", ip)

	require.NotEqual(t, "内网", addr, "expected non-internal addr for public ip, got %s", addr)
}

func TestGetClientIPAndAddrNormalizesMappedIPv6(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	ctx.Request.RemoteAddr = "127.0.0.1:1"
	ctx.Request.Header.Set("X-Real-IP", "::ffff:198.51.100.20")

	ip, _ := GetClientIPAndAddr(ctx, &stubRegionResolver{region: "美国"})
	require.Equal(t, "198.51.100.20", ip, "expected normalized ipv4, got %s", ip)
}

// TestGetClientIPAndAddrPrivateIPSkipsResolver 保证内网 IP 不查询归属地，
// 与原 GetIpAddr 先做 HasLocalIPAddr 判定的行为一致。
func TestGetClientIPAndAddrPrivateIPSkipsResolver(t *testing.T) {
	t.Parallel()

	resolver := &stubRegionResolver{region: "不应被使用"}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	ctx.Request.RemoteAddr = "127.0.0.1:1"
	ctx.Request.Header.Set("X-Real-IP", "10.1.2.3")

	ip, addr := GetClientIPAndAddr(ctx, resolver)
	require.Equal(t, "10.1.2.3", ip, "expected private ip, got %s", ip)

	require.Equal(t, "内网", addr, "expected 内网, got %s", addr)

	require.Empty(t, resolver.calls, "resolver should not be queried for private ip, got %v", resolver.calls)
}

// TestGetClientIPAndAddrNilResolver 保证 resolver 缺失时公网 IP 返回「未知地址」，
// 与原实现中 global.IPSearcher 为 nil 的行为一致。
func TestGetClientIPAndAddrNilResolver(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	ctx.Request.RemoteAddr = "8.8.4.4:1"

	ip, addr := GetClientIPAndAddr(ctx, nil)
	require.Equal(t, "8.8.4.4", ip, "got ip=%q addr=%q, want 8.8.4.4/未知地址", ip, addr)
	require.Equal(t, "未知地址", addr, "got ip=%q addr=%q, want 8.8.4.4/未知地址", ip, addr)
}

func TestNormalizeIPString(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		//nolint:gocritic // Whitespace deliberately tests input normalization.
		" 8.8.4.4 ":         "8.8.4.4",
		"8.8.4.4:443":       "8.8.4.4",
		"[2001:db8::1]:443": "2001:db8::1",
		"::ffff:1.2.3.4":    "1.2.3.4",
		"not-an-ip":         "",
		"":                  "",
	}
	for in, want := range cases {
		{
			got := normalizeIPString(in)
			require.Equal(t, want, got, "normalizeIPString(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestResolveClientIPRejectsSpoofedHeadersFromUntrustedPeer(t *testing.T) {
	t.Parallel()

	ctx, engine := gin.CreateTestContext(httptest.NewRecorder())
	{
		err := engine.SetTrustedProxies([]string{"127.0.0.1"})
		require.NoError(t, err)
	}

	ctx.Request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

	ctx.Request.RemoteAddr = "198.51.100.10:1234"
	for _, header := range []string{"CF-Connecting-IP", "True-Client-IP", "X-Real-IP", "X-Forwarded-For"} {
		ctx.Request.Header.Set(header, "203.0.113.99")
	}

	{
		got := ResolveClientIP(ctx)
		require.Equal(t, "198.51.100.10", got, "spoofed header trusted: %s", got)
	}
}
