package ai

import (
	"maps"
	"strings"
)

// maxImageResultDepth 递归统计图片结果时的最大深度（防御异常深层结构）。
const maxImageResultDepth = 8

// imageGenerationPayload 是上游出图响应。解析和合并留在应用层，领域只接收有业务含义的值。
type imageGenerationPayload struct {
	raw any
}

// partialResult 保留已经成功的图片；不向客户端暴露补图失败的上游细节。
func (payload imageGenerationPayload) partialResult(want int) map[string]any {
	result := make(map[string]any)
	if original, ok := payload.raw.(map[string]any); ok {
		maps.Copy(result, original)
	}
	result["partial"] = true
	result["requestedCount"] = want
	result["returnedCount"] = payload.count()
	result["message"] = "部分图片未生成，已保留成功结果"
	return result
}

func (payload imageGenerationPayload) count() int {
	if items := payload.dataItems(); len(items) > 0 {
		return len(items)
	}

	return payload.countValues(0)
}

func (payload imageGenerationPayload) dataItems() []any {
	root, ok := payload.raw.(map[string]any)
	if !ok {
		return nil
	}

	items, ok := root["data"].([]any)
	if !ok {
		return nil
	}

	return items
}

// merge 把补充请求的出图合并进已有结果，并裁剪到 limit。
//
// 返回值第二项表示是否真的增长（false 时调用方应停止补图，避免死循环）。
func (base imageGenerationPayload) merge(extra imageGenerationPayload, limit int) (any, bool) {
	baseItems := base.dataItems()
	if len(baseItems) == 0 {
		if baseMap, ok := base.raw.(map[string]any); ok && (imageGenerationPayload{raw: baseMap}).hasDirectImage() {
			baseItems = []any{baseMap}
		}
	}

	extraItems := extra.dataItems()
	if len(extraItems) == 0 {
		if extraMap, ok := extra.raw.(map[string]any); ok && (imageGenerationPayload{raw: extraMap}).hasDirectImage() {
			extraItems = []any{extraMap}
		}
	}

	if len(extraItems) == 0 {
		return base.raw, false
	}

	if limit <= 0 {
		limit = len(baseItems) + len(extraItems)
	}

	if len(baseItems) >= limit {
		return base.raw, false
	}

	mergedItems := make([]any, 0, len(baseItems)+len(extraItems))

	mergedItems = append(mergedItems, baseItems...)

	for _, item := range extraItems {
		if len(mergedItems) >= limit {
			break
		}

		mergedItems = append(mergedItems, item)
	}

	if len(mergedItems) == len(baseItems) {
		return base.raw, false
	}

	if root, ok := base.raw.(map[string]any); ok {
		merged := make(map[string]any, len(root)+1)
		maps.Copy(merged, root)

		merged["data"] = mergedItems

		return merged, true
	}

	return map[string]any{"data": mergedItems}, true
}

func (payload imageGenerationPayload) hasDirectImage() bool {
	node, ok := payload.raw.(map[string]any)
	if !ok {
		return false
	}

	for _, key := range []string{"url", contentTypeImageURL, "imageUrl", "b64_json"} {
		if value, ok := node[key].(string); ok && strings.TrimSpace(value) != "" {
			return true
		}
	}

	return false
}

func (payload imageGenerationPayload) countValues(depth int) int {
	if depth > maxImageResultDepth || payload.raw == nil {
		return 0
	}

	switch value := payload.raw.(type) {
	case []any:
		total := 0
		for _, item := range value {
			total += imageGenerationPayload{raw: item}.countValues(depth + 1)
		}

		return total
	case map[string]any:
		if (imageGenerationPayload{raw: value}).hasDirectImage() {
			return 1
		}

		total := 0
		for _, item := range value {
			total += imageGenerationPayload{raw: item}.countValues(depth + 1)
		}

		return total
	default:
		return 0
	}
}
