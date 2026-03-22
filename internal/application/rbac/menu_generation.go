package rbac

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"
)

// parseMenuSuggestion 将上游文本翻译为本上下文模型，并校验菜单约束。
//
// 提示词中的“只返回安全 JSON”不是约束执行机制。模型输出仍需反序列化、必填/长度检查和 SVG 安全校验。
// 这里返回候选字段，实际创建菜单还要经过父级、唯一性与持久化用例，不能跳过常规写入规则。
func parseMenuSuggestion(text string) (*rbacdomain.MenuSuggestion, error) {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return nil, errors.New("AI 未返回合法的菜单 JSON")
	}

	var suggestion rbacdomain.MenuSuggestion
	if err := json.Unmarshal([]byte(text[start:end+1]), &suggestion); err != nil {
		return nil, fmt.Errorf("解析 AI 菜单 JSON 失败: %w", err)
	}

	normalized, err := suggestion.Normalize()
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}
