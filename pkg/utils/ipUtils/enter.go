// Package ipUtils 组合 IP 识别、地址分类和地区查询。
//
// 本地/公网地址探测与请求 IP 归属地是不同用途，涉及外部查询的函数要检查超时和回退。
// RegionResolver 通过端口注入，避免每次调用都构造 ip2region 资源；代理来源可信度由 HTTP 层配置负责。
package ipUtils

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetLocalIPv4 枚举本机网卡并返回首个非回环 IPv4；不保证它就是服务对外可达的地址。
func GetLocalIPv4() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", errors.New("no ipv4 found")
}

// HasLocalIPAddr 解析文本 IP 后按回环、私网、链路本地或未指定地址规则判断。
func HasLocalIPAddr(ip string) bool {
	return HasLocalIP(net.ParseIP(ip))
}

// HasLocalIP 检测 IP 地址是否是内网地址, 通过直接对比 ipUtils 段范围效率更高.
func HasLocalIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	if ip.IsLoopback() {
		return true
	}

	// 链路本地 / 私有网段 / 未指定地址
	if ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}

	ip4 := ip.To4()
	if ip4 == nil { // ipv6 非私有
		return false
	}

	return ip4[0] == 10 || // 10.0.0.0/8
		(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) || // 172.16.0.0/12
		(ip4[0] == 169 && ip4[1] == 254) || // 169.254.0.0/16
		(ip4[0] == 192 && ip4[1] == 168) // 192.168.0.0/16
}

// GetPublicIP 向外部 IP 服务查询本机出网地址，使用独立短超时；它不是当前请求客户端的 IP。
func GetPublicIP() (string, error) {
	client := &http.Client{Timeout: ctxutil.ExternalShortTimeout}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://ipinfo.io/ip", nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil || resp == nil {
		return "", err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("get public ip failed: %s", resp.Status)
	}

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(ip), nil
}

// RegionResolver 归属地解析端口，由基础设施层的 ip2region 适配器实现。
// 这样 pkg 与 interfaces 层无需依赖具体的 xdb 实现，也不再需要全局单例。
type RegionResolver interface {
	// Region 返回 IP 对应的归属地描述。
	Region(ip string) string
}

// GetIPAndAddr 兼容旧调用入口，现直接解析请求客户端 IP 并用注入的归属地查询器获取地区。
func GetIPAndAddr(c *gin.Context, resolver RegionResolver) (ip, addr string) {
	// 保持兼容旧调用方，但实际记录客户端 IP，避免请求外部服务导致接口阻塞。
	return GetClientIPAndAddr(c, resolver)
}

// GetClientIPAndAddr 获取客户端IP及归属地（不依赖外部公网 IP 服务）.
// 优先读取代理/CDN 注入的真实客户端 IP 头，再回退到 gin.ClientIP / RemoteAddr。
func GetClientIPAndAddr(c *gin.Context, resolver RegionResolver) (ip, addr string) {
	ip = ResolveClientIP(c)
	if ip == "" || ip == "::1" {
		return "127.0.0.1", "内网"
	}

	if HasLocalIPAddr(ip) {
		return ip, "内网"
	}

	if resolver == nil {
		return ip, "未知地址"
	}

	return ip, resolver.Region(ip)
}

// ResolveClientIP 从请求中解析真实客户端 IP。
func ResolveClientIP(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}

	// 1) 优先使用显式转发头（开发 Vite 代理 / Nginx / CDN 应写入）
	for _, header := range [...]string{
		"CF-Connecting-IP", // Cloudflare
		"True-Client-IP",   // Akamai / 部分 CDN
		"X-Real-IP",        // Nginx 常用
		"X-Forwarded-For",  // 标准转发链（取最左侧原始客户端）
	} {
		if candidate := firstValidIPFromHeader(c.GetHeader(header)); candidate != "" {
			return candidate
		}
	}

	// 2) 回退 gin.ClientIP（会结合 TrustedProxies 解析 X-Forwarded-For / X-Real-IP）
	if candidate := normalizeIPString(c.ClientIP()); candidate != "" {
		return candidate
	}

	// 3) 最后使用 RemoteAddr
	if host, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr)); err == nil {
		return normalizeIPString(host)
	}

	return normalizeIPString(c.Request.RemoteAddr)
}

// firstValidIPFromHeader 从单个 header 中解析第一个合法 IP。
// X-Forwarded-For 形如 "client, proxy1, proxy2"，取最左侧客户端。
func firstValidIPFromHeader(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}

	parts := strings.Split(header, ",")
	for _, part := range parts {
		if ip := normalizeIPString(part); ip != "" {
			return ip
		}
	}

	return ""
}

// normalizeIPString 清洗 IP 字符串：去空白、去端口、IPv6 括号、IPv4-mapped IPv6。
func normalizeIPString(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// "[::1]:12345" 或 "1.2.3.4:5678"
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}

	raw = strings.Trim(raw, "[]")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	ip := net.ParseIP(raw)
	if ip == nil {
		return ""
	}

	// ::ffff:1.2.3.4 → 1.2.3.4
	if ip4 := ip.To4(); ip4 != nil {
		return ip4.String()
	}

	return ip.String()
}
