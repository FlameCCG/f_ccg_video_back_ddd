// Package wsorigin 校验浏览器 WebSocket 的 Origin 头（原 pkg/wsorigin）。
//
// 原实现是包级函数 + 直读 global.Config()；新架构改为注入 *config.Provider 的
// Checker 结构体，判定逻辑与原实现逐行一致。Origin 校验属于接口层职责，
// 因此归入 interfaces/ws。
package wsorigin

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"f_ccg_video_back_ddd/internal/infrastructure/config"
	third_login "f_ccg_video_back_ddd/internal/infrastructure/config/thirdlogin"
)

// Checker 基于当前配置快照校验 WebSocket Origin。
type Checker struct {
	cfgProvider *config.Provider
}

// NewChecker 构造 Origin 校验器。
func NewChecker(cfgProvider *config.Provider) *Checker {
	return &Checker{cfgProvider: cfgProvider}
}

// Check 校验 Origin：允许同源、双方回环，或命中配置中的前端回调域名。
func (c *Checker) Check(r *http.Request) bool {
	if r == nil {
		return false
	}

	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return false
	}

	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	if sameHost(parsed.Host, r.Host) {
		return true
	}

	if isLoopback(parsed.Hostname()) && isLoopback(hostnameOnly(r.Host)) {
		return true
	}

	return c.configuredOriginAllowed(parsed)
}

// configuredOriginAllowed 比对配置中 5 家三方登录的 redirect 域名。
func (c *Checker) configuredOriginAllowed(origin *url.URL) bool {
	if origin == nil || c == nil {
		return false
	}

	cfg := c.cfgProvider.Get()
	if cfg == nil || cfg.ThirdLogin == nil {
		return false
	}

	for _, raw := range []string{
		redirectOf(cfg.ThirdLogin.GitHub),
		redirectOf(cfg.ThirdLogin.Google),
		redirectOf(cfg.ThirdLogin.LinuxDo),
		redirectOf(cfg.ThirdLogin.X),
		redirectOf(cfg.ThirdLogin.QQ),
	} {
		allowed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || allowed.Scheme == "" || allowed.Host == "" {
			continue
		}

		if strings.EqualFold(allowed.Scheme, origin.Scheme) && sameHost(allowed.Host, origin.Host) {
			return true
		}
	}

	return false
}

// redirectOf 从三方登录配置中取回调地址。
//
// 形参是 any 而非具体类型：`ThirdLogin` 的 5 个字段类型各异但都只需要 Redirect 字段。
// 注意每个分支都必须判 `item == nil`——配置里缺失某家三方登录时该字段是 typed nil 指针，
// 装进 any 之后 `v == nil` 为 false，直接取字段会 panic（见 report.md FIND-013）。
func redirectOf(v any) string {
	if v == nil {
		return ""
	}

	switch item := v.(type) {
	case *third_login.GitHub:
		if item == nil {
			return ""
		}

		return item.Redirect
	case *third_login.Google:
		if item == nil {
			return ""
		}

		return item.Redirect
	case *third_login.LinuxDo:
		if item == nil {
			return ""
		}

		return item.Redirect
	case *third_login.X:
		if item == nil {
			return ""
		}

		return item.Redirect
	case *third_login.QQ:
		if item == nil {
			return ""
		}

		return item.Redirect
	default:
		return ""
	}
}

func sameHost(a, b string) bool {
	aHost, aPort := splitHostPort(a)
	bHost, bPort := splitHostPort(b)

	return strings.EqualFold(aHost, bHost) && strings.EqualFold(aPort, bPort)
}

func hostnameOnly(host string) string {
	name, _ := splitHostPort(host)

	return name
}

func splitHostPort(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}

	host, port, err := net.SplitHostPort(value)
	if err == nil {
		return strings.Trim(strings.ToLower(host), "[]"), port
	}

	if strings.Contains(value, ":") && strings.Count(value, ":") == 1 {
		parts := strings.SplitN(value, ":", 2)

		return strings.ToLower(parts[0]), parts[1]
	}

	return strings.Trim(strings.ToLower(value), "[]"), ""
}

func isLoopback(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" {
		return true
	}

	ip := net.ParseIP(host)

	return ip != nil && ip.IsLoopback()
}
