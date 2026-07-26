// Package rbac 提供 RBAC 相关 HTTP 接口（原 api/rbac_api）。
//
// 请求结构体保留在本包（与原 api/rbac_api 同位置语义），
// gin.Context 止步于此：向应用层只传 context.Context 与 DTO。
package rbac

import (
	"strconv"
	"strings"

	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"
	domain "f_ccg_video_back_ddd/internal/domain/rbac"
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
) *Handler {
	return &Handler{roles: roles, menus: menus, permissions: permissions, inherits: inherits}
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
// （SQL 注入面，见 report.md）；此处统一显式解析，非法输入按「记录不存在」处理，
// 合法输入的可观测行为与原实现完全一致。
func parsePositiveID(raw string) (uint, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, false
	}

	return uint(value), true
}
