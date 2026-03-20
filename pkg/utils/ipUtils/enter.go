// Package ipUtils 解析请求客户端 IP、地址分类和地区查询。
//
// RegionResolver 通过端口注入，避免每次调用都构造 ip2region 资源；代理来源可信度由 HTTP 层配置负责。
package ipUtils

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

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

// RegionResolver 归属地解析端口，由基础设施层的 ip2region 适配器实现。
// pkg 与 interfaces 通过端口使用查询器，无需依赖 xdb 具体实现。
type RegionResolver interface {
	// Region 返回 IP 对应的归属地描述。
	Region(ip string) string
}

// GetIPAndAddr 解析请求客户端 IP，并通过注入的归属地查询器获取地区。
func GetIPAndAddr(c *gin.Context, resolver RegionResolver) (ip, addr string) {
	// 记录客户端 IP，不请求外部服务，避免阻塞接口。
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

	// ClientIP applies Gin's configured TrustedProxies boundary.
	return normalizeIPString(c.ClientIP())
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
