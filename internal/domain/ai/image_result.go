package ai

import (
	"maps"
	"strings"
)

// maxImageResultDepth 递归统计图片结果时的最大深度（防御异常深层结构）。
const maxImageResultDepth = 8

// RequestedImageCount 返回请求期望的出图数量（缺省或非法时按 1 计）。
func RequestedImageCount(n *int) int {
	if n == nil || *n <= 1 {
		return 1
	}

	return *n
}

// ImageResultCount 统计上游返回的图片条数。
func ImageResultCount(payload any) int {
	if items := ImageGenerationDataItems(payload); len(items) > 0 {
		return len(items)
	}

	return countImageResultValues(payload, 0)
}

// ImageGenerationDataItems 取出上游 payload 的 data 数组。
func ImageGenerationDataItems(payload any) []any {
	root, ok := payload.(map[string]any)
	if !ok {
		return nil
	}

	items, ok := root["data"].([]any)
	if !ok {
		return nil
	}

	return items
}

// MergeImageGenerationResponses 把补充请求的出图合并进已有结果，并裁剪到 limit。
//
// 返回值第二项表示是否真的增长（false 时调用方应停止补图，避免死循环）。
func MergeImageGenerationResponses(base, extra any, limit int) (any, bool) {
	baseItems := ImageGenerationDataItems(base)
	if len(baseItems) == 0 {
		if baseMap, ok := base.(map[string]any); ok && hasDirectImageResult(baseMap) {
			baseItems = []any{baseMap}
		}
	}

	extraItems := ImageGenerationDataItems(extra)
	if len(extraItems) == 0 {
		if extraMap, ok := extra.(map[string]any); ok && hasDirectImageResult(extraMap) {
			extraItems = []any{extraMap}
		}
	}

	if len(extraItems) == 0 {
		return base, false
	}

	if limit <= 0 {
		limit = len(baseItems) + len(extraItems)
	}

	if len(baseItems) >= limit {
		return base, false
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
		return base, false
	}

	if root, ok := base.(map[string]any); ok {
		merged := make(map[string]any, len(root)+1)
		maps.Copy(merged, root)

		merged["data"] = mergedItems

		return merged, true
	}

	return map[string]any{"data": mergedItems}, true
}

func hasDirectImageResult(node map[string]any) bool {
	for _, key := range []string{"url", ContentTypeImageURL, "imageUrl", "b64_json"} {
		if value, ok := node[key].(string); ok && strings.TrimSpace(value) != "" {
			return true
		}
	}

	return false
}

func countImageResultValues(node any, depth int) int {
	if depth > maxImageResultDepth || node == nil {
		return 0
	}

	switch value := node.(type) {
	case []any:
		total := 0
		for _, item := range value {
			total += countImageResultValues(item, depth+1)
		}

		return total
	case map[string]any:
		if hasDirectImageResult(value) {
			return 1
		}

		total := 0
		for _, item := range value {
			total += countImageResultValues(item, depth+1)
		}

		return total
	default:
		return 0
	}
}
