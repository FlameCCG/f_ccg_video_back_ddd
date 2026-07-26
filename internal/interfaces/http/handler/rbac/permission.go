package rbac

import (
	"strings"

	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// AssignPermissionReq 分配/移除角色权限请求。
type AssignPermissionReq struct {
	RoleName    string           `json:"roleName" binding:"required" label:"角色名称"`
	Permissions []PermissionItem `json:"permissions" binding:"required" label:"权限列表"`
}

// UpdateRolePermissionsReq 替换角色权限请求。
type UpdateRolePermissionsReq struct {
	RoleID uint `json:"roleId" binding:"required,gt=0" label:"角色ID"`
	// Permissions 最终直接权限列表（为空则清空直接权限）
	Permissions []PermissionItem `json:"permissions" label:"权限列表"`
	// BreakInheritance 为 true 时：清除该角色全部继承关系与继承菜单，再写入权限。
	// 用于「取消勾选通配符/继承覆盖的具体 API」场景——不能既继承又缺少其一部分权限。
	BreakInheritance bool `json:"breakInheritance" label:"是否解除继承"`
}

// AssignPermissionView 分配权限.
func (h *Handler) AssignPermissionView(c *gin.Context) {
	req := middlewares.GetBound[AssignPermissionReq](c)

	if err := h.permissions.Assign(c.Request.Context(), rbacapp.AssignPermissionParams{
		RoleName:    req.RoleName,
		Permissions: toDomainPermissions(req.Permissions),
	}); err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithMsg("添加成功", c)
}

// RemovePermissionView 移除权限.
func (h *Handler) RemovePermissionView(c *gin.Context) {
	req := middlewares.GetBound[AssignPermissionReq](c)

	if err := h.permissions.Remove(c.Request.Context(), rbacapp.AssignPermissionParams{
		RoleName:    req.RoleName,
		Permissions: toDomainPermissions(req.Permissions),
	}); err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithMsg("移除成功", c)
}

// UpdateRolePermissionsView 替换角色权限列表.
func (h *Handler) UpdateRolePermissionsView(c *gin.Context) {
	req := middlewares.GetBound[UpdateRolePermissionsReq](c)

	if err := h.permissions.UpdateRolePermissions(c.Request.Context(), rbacapp.UpdateRolePermissionsParams{
		RoleID:           req.RoleID,
		Permissions:      toDomainPermissions(req.Permissions),
		BreakInheritance: req.BreakInheritance,
	}); err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithMsg("更新成功", c)
}

// GetRolePermissionsView 获取角色权限（直接权限 + 有效权限 + 覆盖的具体 API）.
// Query: name
func (h *Handler) GetRolePermissionsView(c *gin.Context) {
	roleName := c.Query("name")
	if strings.TrimSpace(roleName) == "" {
		res.FailWithMsg("角色名称不能为空", c)
		return
	}

	resp, err := h.permissions.RolePermissions(c.Request.Context(), roleName)
	if err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithData(resp, c)
}

// WildcardListView 返回后端所有已配置的通配符权限列表（跨角色去重）.
// 供前端下拉选择；也支持前端手动输入列表中不存在的路径。
func (h *Handler) WildcardListView(c *gin.Context) {
	items, err := h.permissions.WildcardList()
	if err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithData(items, c)
}

// ResourceListView 返回可用于权限配置的资源列表.
func (h *Handler) ResourceListView(c *gin.Context) {
	items, err := h.permissions.ResourceList(c.Request.Context())
	if err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithData(items, c)
}
