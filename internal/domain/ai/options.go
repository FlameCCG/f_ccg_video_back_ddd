package ai

import (
	"fmt"
	"strings"
)

// Option 模型/思考强度的可选项（label + value）。
//
// 与配置结构 config.AIModel 同形（共享内核 internal/config），领域层用本类型避免依赖任何配置包。
type Option struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Options 是一组模型或思考强度选项。
type Options []Option

// Clone 复制并规范化选项列表：丢弃空 value，label 缺省回落 value。
//
// 返回值恒为非 nil 切片，保证序列化为 [] 而不是 null。
func (options Options) Clone() []Option {
	if len(options) == 0 {
		return []Option{}
	}

	out := make([]Option, 0, len(options))

	for _, item := range options {
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
// 空值或包含 grok 的请求一律回落后台默认模型。
func (options Options) NormalizeConfiguredModel(requested, configured, kind string) (string, error) {
	fallback := strings.TrimSpace(configured)

	requested = strings.TrimSpace(requested)
	if requested == "" || strings.Contains(strings.ToLower(requested), "grok") {
		return fallback, nil
	}

	allowed := map[string]struct{}{fallback: {}}

	for _, item := range options {
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
func (options Options) NormalizeConfiguredOption(
	requested string,
	configured string,
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
		requested = "high"
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
		case "high", "max":
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
