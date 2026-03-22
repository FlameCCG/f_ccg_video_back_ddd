package mediaurl

import (
	"net/url"
	"strings"
)

// Formatter 维护媒体播放 URL 的存储 HTTP 参数与路径转义契约。
type Formatter struct{}

// EnsurePlayableContentTypeURL 为 MinIO 对象 URL 添加 response-content-type，
// 使 octet-stream 对象在 nosniff 模式下也能被 Chrome 播放。
func (Formatter) PlayableContentTypeURL(fileURL string) string {
	raw := strings.TrimSpace(fileURL)
	if raw == "" {
		return raw
	}

	low := strings.ToLower(raw)
	if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") {
		return raw
	}

	if strings.Contains(low, "response-content-type=") {
		return (Formatter{}).NormalizeURL(raw)
	}

	base := raw
	if idx := strings.IndexByte(low, '?'); idx >= 0 {
		base = raw[:idx]
	}

	ext := strings.ToLower((Formatter{}).urlExt(base))

	var ct string

	switch ext {
	case ".mp4":
		ct = "video/mp4"
	case ".m3u8":
		ct = "application/x-mpegURL"
	case ".ts":
		ct = "video/mp2t"
	default:
		return (Formatter{}).NormalizeURL(raw)
	}

	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}

	return (Formatter{}).NormalizeURL(raw + sep + "response-content-type=" + url.QueryEscape(ct))
}

// urlExt 取 URL 路径的扩展名（不引入 path/filepath：URL 路径统一用 /）。
func (Formatter) urlExt(p string) string {
	if p == "" {
		return ""
	}

	if i := strings.LastIndexByte(p, '.'); i >= 0 {
		if j := strings.LastIndexByte(p, '/'); j >= 0 && j > i {
			return ""
		}

		return p[i:]
	}

	return ""
}

// NormalizeMediaURL 规范化媒体 URL 的转义路径；解析失败时原样返回。
func (Formatter) NormalizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return raw
	}

	escaped := (&url.URL{Path: u.Path}).EscapedPath()
	if escaped != "" {
		u.RawPath = escaped
	}

	return u.String()
}
