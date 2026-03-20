// Package rbac 把角色、菜单、权限和继承关系的 HTTP 请求翻译为应用用例调用。
//
// handler.go 声明显式依赖，其他文件按入口拆分；参数结构体保留 Gin 绑定与校验标签。
// 从请求取得参数和经过鉴权的身份，构造应用参数后传 c.Request.Context()，最后映射错误和响应。
// gin.Context 止步于接口层，领域规则、事务、缓存与跨上下文调用由 application 编排。
// 具体入口与权限链到 internal/interfaces/http/router 中查看，业务允许条件还需由应用与领域判断。
// JSON 字段、状态码和响应文案属于对外契约。
package rbac

import (
	"strconv"
	"strings"

	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"
	rbacvo "f_ccg_video_back_ddd/internal/domain/rbac/valueobject"

	"go.uber.org/zap"
)

// Handler 持有 rbac 各应用服务。
type Handler struct {
	roles       *rbacapp.RoleService
	menus       *rbacapp.MenuService
	permissions *rbacapp.PermissionService
	inherits    *rbacapp.InheritService
	logger      *zap.Logger
}

// NewHandler 构造 RBAC HTTP 处理器。
func NewHandler(
	roles *rbacapp.RoleService,
	menus *rbacapp.MenuService,
	permissions *rbacapp.PermissionService,
	inherits *rbacapp.InheritService,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		roles:       roles,
		menus:       menus,
		permissions: permissions,
		inherits:    inherits,
		logger:      loggerOrNop(logger),
	}
}

func (h *Handler) log() *zap.Logger {
	if h == nil {
		return zap.NewNop()
	}

	return loggerOrNop(h.logger)
}

func loggerOrNop(logger *zap.Logger) *zap.Logger {
	if logger == nil {
		return zap.NewNop()
	}

	return logger
}

// PermissionItem 权限项（资源路径 + 操作方法），请求体中使用。
type PermissionItem struct {
	Resource string `json:"resource" binding:"required" label:"资源路径"`
	Action   string `json:"action" binding:"required" label:"操作方法"`
}

// toDomainPermissions 把请求体权限项转为领域值对象。
func toDomainPermissions(items []PermissionItem) []rbacvo.PermissionItem {
	if items == nil {
		return nil
	}

	result := make([]rbacvo.PermissionItem, 0, len(items))
	for _, item := range items {
		result = append(result, rbacvo.PermissionItem{Resource: item.Resource, Action: item.Action})
	}

	return result
}

// parsePositiveID 解析路径或查询参数中的正整数主键。
//
// 先显式解析再传入查询，避免非数字输入被当作 SQL 条件；非法输入按记录不存在处理。
func parsePositiveID(raw string) (uint, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, false
	}

	return uint(value), true
}
