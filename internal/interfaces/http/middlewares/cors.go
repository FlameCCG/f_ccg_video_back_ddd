package middlewares

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"f_ccg_video_back_ddd/internal/config"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/gin-gonic/gin"
)

// Cors 基于配置白名单设置跨域头；未配置时仅允许同源/loopback（debug 模式额外放宽本机）。
// 配置由注入的 *infraconfig.Provider 提供，支持运行期热更新（原实现读 global.Config()）。
func Cors(cfgProvider *infraconfig.Provider) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" {
			if allowed, allowCreds := corsOriginAllowed(cfgProvider.Get(), origin, c.Request.Host); allowed {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
				if allowCreds {
					c.Header("Access-Control-Allow-Credentials", "true")
				} else {
					c.Header("Access-Control-Allow-Credentials", "false")
				}
			}
			// 非白名单 Origin：不回写 ACAO，浏览器将拒绝跨域读响应。
		}

		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE, PATCH")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Length, X-CSRF-Token, Token,session,X_Requested_With,Accept, Origin, Host, Connection, Accept-Encoding, Accept-Language,DNT, X-CustomHeader, Keep-Alive, User-Agent, X-Requested-With, If-Modified-Since, Cache-Control, Content-Type, Pragma")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers,Cache-Control,Content-Language,Content-Type,Expires,Last-Modified,Pragma,FooBar")
		c.Header("Access-Control-Max-Age", "172800")

		if c.Request.Method == http.MethodOptions {
			// 预检：非白名单 Origin 返回 403，明确拒绝。
			if origin != "" {
				if allowed, _ := corsOriginAllowed(cfgProvider.Get(), origin, c.Request.Host); !allowed {
					c.AbortWithStatus(http.StatusForbidden)
					return
				}
			}

			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func corsOriginAllowed(cfg *config.Config, origin, requestHost string) (allowed bool, allowCredentials bool) {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false, false
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false, false
	}

	// 同源始终允许。
	if sameHost(parsed.Host, requestHost) {
		return true, true
	}

	if cfg != nil && cfg.Server != nil {
		for _, raw := range cfg.Server.CorsOrigins {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}

			if raw == "*" {
				// 显式配置 * 时放开（不带 credentials）。
				return true, false
			}

			allowedURL, err := url.Parse(raw)
			if err != nil || allowedURL.Host == "" {
				// 允许只写 host:port
				if strings.EqualFold(raw, parsed.Host) || strings.EqualFold(raw, parsed.Hostname()) {
					return true, true
				}

				continue
			}

			if strings.EqualFold(allowedURL.Scheme, parsed.Scheme) && sameHost(allowedURL.Host, parsed.Host) {
				return true, true
			}

			if allowedURL.Scheme == "" && sameHost(allowedURL.Host, parsed.Host) {
				return true, true
			}
		}

		// debug/test 模式：未配置白名单时允许 loopback 前端跨端口联调。
		mode := strings.ToLower(strings.TrimSpace(cfg.Server.Mode))
		if (mode == "debug" || mode == "test" || mode == "dev") &&
			len(cfg.Server.CorsOrigins) == 0 &&
			isLoopbackHost(parsed.Hostname()) {
			return true, true
		}
	}

	return false, false
}

func sameHost(a, b string) bool {
	aHost, aPort := splitHostPort(a)
	bHost, bPort := splitHostPort(b)

	return strings.EqualFold(aHost, bHost) && strings.EqualFold(aPort, bPort)
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

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" {
		return true
	}

	ip := net.ParseIP(host)

	return ip != nil && ip.IsLoopback()
}
