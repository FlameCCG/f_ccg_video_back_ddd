package wsorigin

import (
	"net/http"
	"testing"

	"f_ccg_video_back_ddd/internal/config"
	third_login "f_ccg_video_back_ddd/internal/config/thirdlogin"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
)

// 对应原 pkg/wsorigin/enter.go（原为包级函数 + global.Config()，此处为注入式 Checker）。
// Origin 校验是 WebSocket 的跨站防护，判定结果属于安全契约。

// newRequest 造一个带 Origin 与 Host 的请求。
func newRequest(origin, host string) *http.Request {
	r := &http.Request{Header: make(http.Header), Host: host}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}

	return r
}

// newChecker 用给定三方登录配置构造校验器。
func newChecker(third *config.ThirdLogin) *Checker {
	return NewChecker(infraconfig.NewProvider(&config.Config{ThirdLogin: third}))
}

func TestCheckRejectsMissingOrMalformedOrigin(t *testing.T) {
	t.Parallel()

	checker := newChecker(nil)

	tests := []struct {
		name    string
		request *http.Request
	}{
		{name: "nil request", request: nil},
		{name: "no origin header", request: newRequest("", "example.com")},
		{name: "blank origin", request: newRequest("   ", "example.com")},
		{name: "origin without host", request: newRequest("http://", "example.com")},
		{name: "non http scheme", request: newRequest("ws://example.com", "example.com")},
		{name: "file scheme", request: newRequest("file:///tmp/x.html", "example.com")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if checker.Check(tt.request) {
				t.Fatal("origin must be rejected")
			}
		})
	}
}

// TestCheckAllowsSameHost 同源（host + port 一致，大小写不敏感）直接放行。
func TestCheckAllowsSameHost(t *testing.T) {
	t.Parallel()

	checker := newChecker(nil)

	tests := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{name: "identical", origin: "https://example.com", host: "example.com", want: true},
		{name: "case insensitive", origin: "https://EXAMPLE.com", host: "example.com", want: true},
		{name: "same host and port", origin: "http://example.com:8080", host: "example.com:8080", want: true},
		{name: "port mismatch", origin: "http://example.com:8080", host: "example.com:9090", want: false},
		{name: "origin has port host does not", origin: "http://example.com:8080", host: "example.com", want: false},
		{name: "different host", origin: "https://evil.com", host: "example.com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := checker.Check(newRequest(tt.origin, tt.host)); got != tt.want {
				t.Fatalf("Check(%q, %q) = %v, want %v", tt.origin, tt.host, got, tt.want)
			}
		})
	}
}

// TestCheckAllowsLoopbackPairs 双方都是回环地址时放行（本地开发跨端口）。
func TestCheckAllowsLoopbackPairs(t *testing.T) {
	t.Parallel()

	checker := newChecker(nil)

	tests := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{name: "localhost to 127.0.0.1", origin: "http://localhost:5173", host: "127.0.0.1:8080", want: true},
		{name: "127.0.0.1 to localhost", origin: "http://127.0.0.1:5173", host: "localhost:8080", want: true},
		{name: "ipv6 loopback", origin: "http://[::1]:5173", host: "localhost:8080", want: true},
		{name: "loopback origin public host", origin: "http://localhost:5173", host: "example.com", want: false},
		{name: "public origin loopback host", origin: "http://evil.com", host: "localhost:8080", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := checker.Check(newRequest(tt.origin, tt.host)); got != tt.want {
				t.Fatalf("Check(%q, %q) = %v, want %v", tt.origin, tt.host, got, tt.want)
			}
		})
	}
}

// TestCheckAllowsConfiguredThirdLoginRedirects 非同源时比对 5 家三方登录的回调域名。
func TestCheckAllowsConfiguredThirdLoginRedirects(t *testing.T) {
	t.Parallel()

	third := &config.ThirdLogin{
		GitHub:  &third_login.GitHub{Redirect: "https://front.example.com/oauth/github"},
		Google:  &third_login.Google{Redirect: " https://google-front.example.com/cb "},
		LinuxDo: &third_login.LinuxDo{Redirect: "https://linuxdo.example.com:8443/cb"},
		X:       &third_login.X{Redirect: "not a url"},
		QQ:      &third_login.QQ{Redirect: ""},
	}
	checker := newChecker(third)

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{name: "github redirect host", origin: "https://front.example.com", want: true},
		{name: "google redirect trimmed", origin: "https://google-front.example.com", want: true},
		{name: "linuxdo redirect with port", origin: "https://linuxdo.example.com:8443", want: true},
		{name: "port must match", origin: "https://linuxdo.example.com", want: false},
		{name: "scheme must match", origin: "http://front.example.com", want: false},
		{name: "unconfigured host", origin: "https://evil.com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := checker.Check(newRequest(tt.origin, "api.example.com")); got != tt.want {
				t.Fatalf("Check(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}

// TestCheckRejectsWhenConfigMissing 配置缺失（nil Config / nil ThirdLogin）时
// 只保留同源与回环放行，其余一律拒绝——不因配置缺失而放宽。
func TestCheckRejectsWhenConfigMissing(t *testing.T) {
	t.Parallel()

	for _, checker := range []*Checker{
		NewChecker(infraconfig.NewProvider(nil)),
		newChecker(nil),
		newChecker(&config.ThirdLogin{}),
	} {
		if checker.Check(newRequest("https://front.example.com", "api.example.com")) {
			t.Fatal("cross-origin request must be rejected without configured redirects")
		}

		// 同源仍然放行（不依赖配置）。
		if !checker.Check(newRequest("https://api.example.com", "api.example.com")) {
			t.Fatal("same-origin request must stay allowed")
		}
	}
}

func TestRedirectOfHandlesAllProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "nil", value: nil, want: ""},
		{name: "github", value: &third_login.GitHub{Redirect: "g"}, want: "g"},
		{name: "google", value: &third_login.Google{Redirect: "o"}, want: "o"},
		{name: "linuxdo", value: &third_login.LinuxDo{Redirect: "l"}, want: "l"},
		{name: "x", value: &third_login.X{Redirect: "x"}, want: "x"},
		{name: "qq", value: &third_login.QQ{Redirect: "q"}, want: "q"},
		{name: "unknown type", value: struct{}{}, want: ""},
		// 配置缺失某家三方登录时字段是 typed nil 指针，装进 any 后 v != nil，
		// 必须逐分支判空，否则取 Redirect 会 panic（report.md FIND-013）。
		{name: "typed nil github", value: (*third_login.GitHub)(nil), want: ""},
		{name: "typed nil google", value: (*third_login.Google)(nil), want: ""},
		{name: "typed nil linuxdo", value: (*third_login.LinuxDo)(nil), want: ""},
		{name: "typed nil x", value: (*third_login.X)(nil), want: ""},
		{name: "typed nil qq", value: (*third_login.QQ)(nil), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := redirectOf(tt.value); got != tt.want {
				t.Fatalf("redirectOf(%T) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestSplitHostPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		wantHost string
		wantPort string
	}{
		{name: "host and port", value: "Example.com:8080", wantHost: "example.com", wantPort: "8080"},
		{name: "host only", value: "Example.com", wantHost: "example.com", wantPort: ""},
		{name: "ipv6 with port", value: "[::1]:8080", wantHost: "::1", wantPort: "8080"},
		{name: "ipv6 without port", value: "[::1]", wantHost: "::1", wantPort: ""},
		{name: "blank", value: "   ", wantHost: "", wantPort: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			host, port := splitHostPort(tt.value)
			if host != tt.wantHost || port != tt.wantPort {
				t.Fatalf("splitHostPort(%q) = (%q, %q), want (%q, %q)",
					tt.value, host, port, tt.wantHost, tt.wantPort)
			}
		})
	}
}

func TestIsLoopback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		host string
		want bool
	}{
		{host: "localhost", want: true},
		{host: "LOCALHOST", want: true},
		{host: "127.0.0.1", want: true},
		{host: "127.5.5.5", want: true},
		{host: "[::1]", want: true},
		{host: "example.com", want: false},
		{host: "10.0.0.1", want: false},
		{host: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			t.Parallel()

			if got := isLoopback(tt.host); got != tt.want {
				t.Fatalf("isLoopback(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}
