// Package rbac 把角色、菜单、权限和继承关系的 HTTP 请求翻译为应用用例调用。
//
// handler.go 声明显式依赖，其他文件按入口拆分；参数结构体保留 Gin 绑定与校验标签。
// 从请求取得参数和经过鉴权的身份，构造应用参数后传 c.Request.Context()，最后映射错误和响应。
// gin.Context 止步于接口层，领域规则、事务、缓存与跨上下文调用由 application 编排。
// 具体入口与权限链到 internal/interfaces/http/router 中查看，业务允许条件还需由应用与领域判断。
// JSON 字段、状态码和响应文案属于对外契约，新增注释不应顺带调整这些行为。
package rbac

import (
	"strconv"
	"strings"

	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"
	domain "f_ccg_video_back_ddd/internal/domain/rbac"

	"go.uber.org/zap"
)

// Handler 持有 rbac 各应用服务。
type Handler struct {
	roles       *rbacapp.RoleService
	menus       *rbacapp.MenuService
	permissions *rbacapp.PermissionService
	inherits    *rbacapp.InheritService
}

// NewHandler 构造 RBAC HTTP 处理器。
func NewHandler(
	roles *rbacapp.RoleService,
	menus *rbacapp.MenuService,
	permissions *rbacapp.PermissionService,
	inherits *rbacapp.InheritService,
	_ *zap.Logger,
) *Handler {
	return &Handler{
		roles:       roles,
		menus:       menus,
		permissions: permissions,
		inherits:    inherits,
	}
}

// PermissionItem 权限项（资源路径 + 操作方法），请求体中使用。
type PermissionItem struct {
	Resource string `json:"resource" binding:"required" label:"资源路径"`
	Action   string `json:"action" binding:"required" label:"操作方法"`
}

// toDomainPermissions 把请求体权限项转为领域值对象。
func toDomainPermissions(items []PermissionItem) []domain.PermissionItem {
	if items == nil {
		return nil
	}

	result := make([]domain.PermissionItem, 0, len(items))
	for _, item := range items {
		result = append(result, domain.PermissionItem{Resource: item.Resource, Action: item.Action})
	}

	return result
}

// parsePositiveID 解析路径/查询参数中的正整数主键。
//
// 原实现把查询串直接交给 GORM 作为主键条件，非数字输入会被 GORM 当作原始 SQL 片段拼接
// （可扩大 SQL 条件的输入面）；此处统一显式解析，非法输入按「记录不存在」处理，
// 合法输入的可观测行为与原实现完全一致。
func parsePositiveID(raw string) (uint, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, false
	}

	return uint(value), true
}
