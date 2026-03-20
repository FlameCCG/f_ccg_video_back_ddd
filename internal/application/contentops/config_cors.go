package contentops

import (
	"fmt"
	"net/url"
	"strings"
)

// normalizeAdminCorsOrigins 校验并归一化 Origin 列表（允许 "*" 或完整 http(s) URL）。
func normalizeAdminCorsOrigins(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}

	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))

	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		if item == "*" {
			if _, ok := seen["*"]; !ok {
				seen["*"] = struct{}{}
				out = append(out, "*")
			}

			continue
		}

		parsed, err := url.Parse(item)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return nil, fmt.Errorf("无效的 CORS Origin: %s（请使用完整 URL，如 https://www.example.com）", item)
		}

		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return nil, fmt.Errorf("无效的 CORS Origin scheme: %s（仅支持 http/https）", item)
		}

		// 规范化：去掉 path/query，只保留 scheme://host
		normalized := parsed.Scheme + "://" + parsed.Host
		if _, ok := seen[normalized]; ok {
			continue
		}

		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	return out, nil
}
