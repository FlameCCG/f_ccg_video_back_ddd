// Package rbac 提供 RBAC HTTP 接口。
//
// 请求结构体位于接口层，应用层仅接收 context.Context 与 DTO。
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

// parsePositiveID 显式解析路径或查询参数中的正整数主键，
// 避免非数字输入被当作 SQL 条件；非法输入按记录不存在处理。
func parsePositiveID(raw string) (uint, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, false
	}

	return uint(value), true
}
