package rbac

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
