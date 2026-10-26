package ai

import (
	"fmt"
	"strings"
)

// 上游消息协议中的固定取值（同时是请求/响应 JSON 契约的一部分）。
const (
	// ContentTypeImageURL 图片内容块类型。
	ContentTypeImageURL = "image_url"
	// ContentTypeText 文本内容块类型。
	ContentTypeText = "text"
	// RoleSystem 系统角色名。
	RoleSystem = "system"
)

// Option 模型/思考强度的可选项（label + value）。
//
// 与配置结构 config.AIModel 同形（共享内核 internal/config），领域层用本类型避免依赖任何配置包。
type Option struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// CloneOptions 复制并规范化选项列表：丢弃空 value，label 缺省回落 value。
//
// 返回值恒为非 nil 切片，保证序列化为 [] 而不是 null（与原实现一致）。
func CloneOptions(src []Option) []Option {
	if len(src) == 0 {
		return []Option{}
	}

	out := make([]Option, 0, len(src))

	for _, item := range src {
		label := strings.TrimSpace(item.Label)

		value := strings.TrimSpace(item.Value)
		if value == "" {
			continue
		}

		if label == "" {
			label = value
		}

		out = append(out, Option{Label: label, Value: value})
	}

	return out
}

// NormalizeConfiguredModel 校验请求模型是否在「后台默认值 + 白名单」内。
//
// 与原实现一致：空值或包含 grok 的请求一律回落后台默认模型。
func NormalizeConfiguredModel(requested, configured string, models []Option, kind string) (string, error) {
	fallback := strings.TrimSpace(configured)

	requested = strings.TrimSpace(requested)
	if requested == "" || strings.Contains(strings.ToLower(requested), "grok") {
		return fallback, nil
	}

	allowed := map[string]struct{}{fallback: {}}

	for _, item := range models {
		if value := strings.TrimSpace(item.Value); value != "" {
			allowed[value] = struct{}{}
		}
	}

	if _, ok := allowed[requested]; !ok {
		return "", fmt.Errorf("不支持的%s模型: %s", kind, requested)
	}

	return requested, nil
}

// NormalizeConfiguredOption 校验 label/value 白名单选项；空列表时仅允许 fallback.
func NormalizeConfiguredOption(
	requested string,
	configured string,
	options []Option,
	kind string,
	hardDefault string,
) (string, error) {
	fallback := strings.TrimSpace(configured)
	if fallback == "" {
		fallback = strings.TrimSpace(hardDefault)
	}

	requested = strings.TrimSpace(requested)
	if requested == "" {
		return fallback, nil
	}

	// DeepSeek 兼容：low/medium → high，xhigh → max
	switch strings.ToLower(requested) {
	case "low", "medium":
		requested = testHigh
	case "xhigh":
		requested = "max"
	}

	allowed := map[string]struct{}{}
	if fallback != "" {
		allowed[fallback] = struct{}{}
	}

	for _, item := range options {
		if value := strings.TrimSpace(item.Value); value != "" {
			allowed[value] = struct{}{}
		}
	}

	// 无白名单且无默认时，放行已规范化的 high/max
	if len(allowed) == 0 {
		switch requested {
		case testHigh, "max":
			return requested, nil
		default:
			return "", fmt.Errorf("不支持的%s: %s", kind, requested)
		}
	}

	if _, ok := allowed[requested]; !ok {
		return "", fmt.Errorf("不支持的%s: %s", kind, requested)
	}

	return requested, nil
}

// FirstNonEmpty 返回第一个非空（去空格后）字符串。
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}

	return ""
}

// NormalizeBaseURL 去掉基础 URL 末尾的斜杠。
func NormalizeBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

// JoinURL 拼接基础 URL 与路径。
func JoinURL(baseURL, path string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + path
}
