package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	domainai "f_ccg_video_back_ddd/internal/domain/ai"
	domainrbac "f_ccg_video_back_ddd/internal/domain/rbac"

	"github.com/cloudwego/eino/schema"
)

const defaultMenuGenerationSystemPrompt = `你是企业后台 RBAC 菜单配置生成器。你会收到一个中文菜单标题、可选父菜单 ID，以及 menus 表的完整现有数据。请严格参考已有菜单的命名、层级、路由和 SVG 图标风格，为新菜单生成除中文标题和父菜单之外的全部业务字段。

只返回一个合法 JSON 对象，不要使用 Markdown 代码块，不要解释。JSON 必须包含且只包含：
{"titleEn":"英文标题","titleJa":"日文标题","icon":"完整且可直接渲染的 SVG 字符串","path":"路由路径","name":"唯一的 PascalCase 路由名称","component":"前端组件路径或 Layout","keepAlive":false,"sortOrder":1}

规则：
1. 顶级菜单 path 使用以 / 开头的绝对路径，component 通常为 Layout；子菜单 path 使用相对路径，component 使用 views/<模块>/<页面>/index。
2. name、path 和 component 必须结合现有数据保持唯一且风格一致。
3. sortOrder 必须是大于 0 的整数，并结合父菜单下现有同级菜单给出合理插入位置。
4. icon 必须是长度不超过 2048 字节的安全 SVG 字符串，不得包含 script、事件处理器、外链或 foreignObject。
5. titleEn/titleJa/name/path/component 均不得为空。`

var menuSVGEventAttribute = regexp.MustCompile(`(?i)\son[a-z]+\s*=`)

// GenerateMenuSuggestion 使用独立系统提示词生成菜单表单字段。
func (s *Service) GenerateMenuSuggestion(
	ctx context.Context,
	title string,
	parentID *uint,
	menuTableJSON []byte,
) (*domainrbac.MenuSuggestion, error) {
	snapshot, err := s.snapshot()
	if err != nil {
		return nil, err
	}
	cfg := &snapshot.settings

	if err := requireChatAPIKey(cfg); err != nil {
		return nil, err
	}

	model, err := domainai.NormalizeConfiguredModel("", cfg.ChatModel, toOptions(cfg.ChatModels), "聊天")
	if err != nil {
		return nil, err
	}

	systemPrompt := strings.TrimSpace(cfg.MenuGenerationSystemPrompt)
	if systemPrompt == "" {
		systemPrompt = defaultMenuGenerationSystemPrompt
	}

	parentValue := "null（顶级菜单）"
	if parentID != nil && *parentID > 0 {
		parentValue = fmt.Sprintf("%d", *parentID)
	}

	userPrompt := fmt.Sprintf(
		"中文标题：%s\n父菜单 ID：%s\nmenus 表完整数据（JSON）：\n%s",
		strings.TrimSpace(title),
		parentValue,
		menuTableJSON,
	)

	text, err := s.text.InvokeText(ctx, TextInvocation{
		Runtime: snapshot.runtime,
		Model:   model,
		Messages: []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(userPrompt),
		},
		MaxTokens: defaultMaxOutputTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("生成菜单字段失败: %w", err)
	}

	return parseMenuSuggestion(text)
}

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

	iconLower := strings.ToLower(suggestion.Icon)
	if !strings.HasPrefix(iconLower, "<svg") ||
		strings.Contains(iconLower, "<script") ||
		strings.Contains(iconLower, "<foreignobject") ||
		strings.Contains(iconLower, "javascript:") ||
		menuSVGEventAttribute.MatchString(suggestion.Icon) {
		return nil, errors.New("AI 返回了不安全的菜单 SVG")
	}

	return &suggestion, nil
}
