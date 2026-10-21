package ai

import (
	"context"
	"fmt"
	"strings"

	domainai "f_ccg_video_back_ddd/internal/domain/ai"

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

// GenerateMenuSuggestion 使用独立系统提示词生成菜单表单字段。
func (s *Service) GenerateMenuSuggestion(
	ctx context.Context,
	title string,
	parentID *uint,
	menuTableJSON []byte,
) (string, error) {
	snapshot, err := s.snapshot()
	if err != nil {
		return "", err
	}
	cfg := &snapshot.settings

	if err := requireChatAPIKey(cfg); err != nil {
		return "", err
	}

	model, err := domainai.NormalizeConfiguredModel("", cfg.ChatModel, toOptions(cfg.ChatModels), "聊天")
	if err != nil {
		return "", err
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
		return "", fmt.Errorf("生成菜单字段失败: %w", err)
	}

	return text, nil
}
