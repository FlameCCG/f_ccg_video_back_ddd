package rbac

import (
	"context"
)

// MenuSuggestionGenerator AI 菜单字段生成端口。
//
// 接口由消费方 rbac 定义；AI 上下文的应用服务实现它，基础设施聊天模型仍经
// AI 自己的 TextGenerator 端口调用。返回上游文本，由 rbac 解析并执行菜单校验。
type MenuSuggestionGenerator interface {
	GenerateMenuSuggestion(
		ctx context.Context,
		title string,
		parentID *uint,
		menuTableJSON []byte,
	) (string, error)
}
