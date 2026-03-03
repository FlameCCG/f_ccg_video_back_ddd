package rbac

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	domainrbac "f_ccg_video_back_ddd/internal/domain/rbac"
)

// parseMenuSuggestion 将上游文本翻译为本上下文模型，并校验菜单约束。
func parseMenuSuggestion(text string) (*domainrbac.MenuSuggestion, error) {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return nil, errors.New("AI 未返回合法的菜单 JSON")
	}

	var suggestion domainrbac.MenuSuggestion
	if err := json.Unmarshal([]byte(text[start:end+1]), &suggestion); err != nil {
		return nil, fmt.Errorf("解析 AI 菜单 JSON 失败: %w", err)
	}

	suggestion.TitleEn = strings.TrimSpace(suggestion.TitleEn)
	suggestion.TitleJa = strings.TrimSpace(suggestion.TitleJa)
	suggestion.Icon = strings.TrimSpace(suggestion.Icon)
	suggestion.Path = strings.TrimSpace(suggestion.Path)
	suggestion.Name = strings.TrimSpace(suggestion.Name)
	suggestion.Component = strings.TrimSpace(suggestion.Component)

	if suggestion.TitleEn == "" ||
		suggestion.TitleJa == "" ||
		suggestion.Icon == "" ||
		suggestion.Path == "" ||
		suggestion.Name == "" ||
		suggestion.Component == "" ||
		suggestion.SortOrder == 0 {
		return nil, errors.New("AI 返回的菜单字段不完整")
	}

	if len(suggestion.TitleEn) > 100 ||
		len(suggestion.TitleJa) > 100 ||
		len(suggestion.Icon) > 2048 ||
		len(suggestion.Path) > 255 ||
		len(suggestion.Name) > 100 ||
		len(suggestion.Component) > 255 {
		return nil, errors.New("AI 返回的菜单字段超过数据库长度限制")
	}

	// 与手工创建/更新共用同一份安全校验（domainrbac.ValidateMenuIcon）；
	// AI 额外要求图标必须是 SVG。
	if !strings.HasPrefix(strings.ToLower(suggestion.Icon), "<svg") ||
		domainrbac.ValidateMenuIcon(suggestion.Icon) != nil {
		return nil, errors.New("AI 返回了不安全的菜单 SVG")
	}

	return &suggestion, nil
}
