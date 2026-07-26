package rbac

import (
	"strconv"
	"strings"

	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// CreateRoleReq 创建角色请求。
type CreateRoleReq struct {
	Name        string           `json:"name" binding:"required" label:"角色名称"`
	Desc        string           `json:"desc" label:"角色描述"`
	MenuIDs     []uint           `json:"menuIds" label:"菜单ID列表"`
	Permissions []PermissionItem `json:"permissions" label:"权限列表"`
	CopyRoleID  uint             `json:"copyRoleId" label:"模板角色ID"`
}

// UpdateRoleReq 更新角色请求。
type UpdateRoleReq struct {
	RoleID uint    `json:"roleId" binding:"required,gt=0" label:"角色ID"`
	Name   *string `json:"name" label:"角色名称"`
	Desc   *string `json:"desc" label:"角色描述"`
}

// AssignRoleReq 分配/移除用户角色请求。
type AssignRoleReq struct {
	UserID  uint   `json:"userId" binding:"required" label:"用户ID"`
	RoleIDs []uint `json:"roleIds" binding:"required,min=1" label:"角色ID"`
}

// UpdateUserRolesReq 更新用户角色（整体替换）请求。
type UpdateUserRolesReq struct {
	UserID  uint   `json:"userId" binding:"required" label:"用户ID"`
	RoleIDs []uint `json:"roleIds" binding:"required" label:"角色ID列表"`
}

// GetRoleListView 获取角色列表.
func (h *Handler) GetRoleListView(c *gin.Context) {
	roles, err := h.roles.List(c.Request.Context())
	if err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithData(roles, c)
}

// CreateRoleView 创建角色.
func (h *Handler) CreateRoleView(c *gin.Context) {
	req := middlewares.GetBound[CreateRoleReq](c)

	if err := h.roles.Create(c.Request.Context(), rbacapp.CreateRoleParams{
		Name:        req.Name,
		Desc:        req.Desc,
		MenuIDs:     req.MenuIDs,
		Permissions: toDomainPermissions(req.Permissions),
		CopyRoleID:  req.CopyRoleID,
	}); err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithMsg("创建成功", c)
}

// UpdateRoleView 更新角色.
func (h *Handler) UpdateRoleView(c *gin.Context) {
	req := middlewares.GetBound[UpdateRoleReq](c)

	if err := h.roles.Update(c.Request.Context(), rbacapp.UpdateRoleParams{
		RoleID: req.RoleID,
		Name:   req.Name,
		Desc:   req.Desc,
	}); err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithMsg("更新成功", c)
}

// DeleteRoleView 删除角色.
// Query: id
func (h *Handler) DeleteRoleView(c *gin.Context) {
	roleIDStr := strings.TrimSpace(c.Query("id"))
	if roleIDStr == "" {
		res.FailWithMsg("角色ID不能为空", c)
		return
	}

	roleID, ok := parsePositiveID(roleIDStr)
	if !ok {
		res.FailWithMsg("角色不存在", c)
		return
	}

	if err := h.roles.Delete(c.Request.Context(), roleID); err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithMsg("删除成功", c)
}

// AssignRoleView 分配用户角色.
func (h *Handler) AssignRoleView(c *gin.Context) {
	req := middlewares.GetBound[AssignRoleReq](c)

	if err := h.roles.AssignUserRoles(c.Request.Context(), req.UserID, req.RoleIDs); err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithMsg("分配成功", c)
}

// RemoveRoleView 移除用户角色.
func (h *Handler) RemoveRoleView(c *gin.Context) {
	req := middlewares.GetBound[AssignRoleReq](c)

	if err := h.roles.RemoveUserRoles(c.Request.Context(), req.UserID, req.RoleIDs); err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithMsg("移除成功", c)
}

// UpdateUserRolesView 更新用户角色（替换所有角色）.
func (h *Handler) UpdateUserRolesView(c *gin.Context) {
	req := middlewares.GetBound[UpdateUserRolesReq](c)

	if err := h.roles.ReplaceUserRoles(c.Request.Context(), req.UserID, req.RoleIDs); err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithMsg("更新成功", c)
}

// GetUserRolesView 获取用户角色列表.
// Query: userId
func (h *Handler) GetUserRolesView(c *gin.Context) {
	userIDStr := strings.TrimSpace(c.Query("userId"))
	if userIDStr == "" {
		res.FailWithMsg("用户ID不能为空", c)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		res.FailWithMsg("用户ID格式错误", c)
		return
	}

	if userID <= 0 {
		res.FailWithMsg("用户不存在", c)
		return
	}

	roles, err := h.roles.UserRoles(c.Request.Context(), uint(userID))
	if err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithData(roles, c)
}

// UserPermissionsView 获取用户最终权限列表.
// Query: userId
func (h *Handler) UserPermissionsView(c *gin.Context) {
	userIDStr := c.Query("userId")
	if userIDStr == "" {
		res.FailWithMsg("userId 不能为空", c)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil || userID <= 0 {
		res.FailWithMsg("userId 非法", c)
		return
	}

	items, err := h.roles.UserPermissions(c.Request.Context(), uint(userID))
	if err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithData(items, c)
}
