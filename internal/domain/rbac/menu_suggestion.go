package rbac

import (
	"errors"
	"strings"
)

// MenuSuggestion 是创建菜单时由 AI 生成的可编辑字段。
type MenuSuggestion struct {
	TitleEn   string `json:"titleEn"`
	TitleJa   string `json:"titleJa"`
	Icon      string `json:"icon"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Component string `json:"component"`
	KeepAlive bool   `json:"keepAlive"`
	SortOrder uint   `json:"sortOrder"`
}

// Normalize validates generated menu fields without modifying the input.
func (suggestion MenuSuggestion) Normalize() (MenuSuggestion, error) {
	normalized := suggestion
	normalized.TitleEn = strings.TrimSpace(normalized.TitleEn)
	normalized.TitleJa = strings.TrimSpace(normalized.TitleJa)
	normalized.Icon = strings.TrimSpace(normalized.Icon)
	normalized.Path = strings.TrimSpace(normalized.Path)
	normalized.Name = strings.TrimSpace(normalized.Name)
	normalized.Component = strings.TrimSpace(normalized.Component)

	if normalized.TitleEn == "" ||
		normalized.TitleJa == "" ||
		normalized.Icon == "" ||
		normalized.Path == "" ||
		normalized.Name == "" ||
		normalized.Component == "" ||
		normalized.SortOrder == 0 {
		return suggestion, errors.New("AI 返回的菜单字段不完整")
	}

	if len(normalized.TitleEn) > 100 ||
		len(normalized.TitleJa) > 100 ||
		len(normalized.Icon) > 2048 ||
		len(normalized.Path) > 255 ||
		len(normalized.Name) > 100 ||
		len(normalized.Component) > 255 {
		return suggestion, errors.New("AI 返回的菜单字段超过数据库长度限制")
	}

	// 与手工创建/更新共用同一份安全校验（domainrbac.ValidateMenuIcon）；
	// AI 额外要求图标必须是 SVG。
	if !strings.HasPrefix(strings.ToLower(normalized.Icon), "<svg") || MenuIcon(normalized.Icon).Validate() != nil {
		return suggestion, errors.New("AI 返回了不安全的菜单 SVG")
	}

	return normalized, nil
}
