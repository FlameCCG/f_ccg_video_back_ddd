package rbac

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"strconv"
	"strings"

	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// RoleInheritReq 角色继承/解除继承请求。
type RoleInheritReq struct {
	RoleID       uint `json:"roleId" binding:"required,gt=0" label:"角色ID"`
	ParentRoleID uint `json:"parentRoleId" binding:"required,gt=0" label:"父角色ID"`
}

// AssignRoleInheritView 角色继承父角色.
func (h *Handler) AssignRoleInheritView(c *gin.Context) {
	req := middlewares.GetBound[RoleInheritReq](c)

	if err := h.inherits.Assign(c.Request.Context(), req.RoleID, req.ParentRoleID); err != nil {
		res.FailWithMsg(clienterror.Message(err), c)
		return
	}

	res.OkWithMsg("继承成功", c)
}

// RemoveRoleInheritView 移除角色继承.
func (h *Handler) RemoveRoleInheritView(c *gin.Context) {
	req := middlewares.GetBound[RoleInheritReq](c)

	if err := h.inherits.Remove(c.Request.Context(), req.RoleID, req.ParentRoleID); err != nil {
		res.FailWithMsg(clienterror.Message(err), c)
		return
	}

	res.OkWithMsg("移除成功", c)
}

// RoleInheritListView 获取角色继承列表（直接 + 间接）.
// Query: roleId
func (h *Handler) RoleInheritListView(c *gin.Context) {
	roleIDStr := c.Query("roleId")
	if roleIDStr == "" {
		res.FailWithMsg("roleId 不能为空", c)
		return
	}

	roleID, err := strconv.Atoi(roleIDStr)
	if err != nil || roleID <= 0 {
		res.FailWithMsg("roleId 非法", c)
		return
	}

	resp, err := h.inherits.List(c.Request.Context(), uint(roleID))
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)
		return
	}

	res.OkWithData(resp, c)
}

// RoleInheritTreeView 获取角色继承树.
// Query: roleId（可选；有值时只返回该角色相关的子树）
func (h *Handler) RoleInheritTreeView(c *gin.Context) {
	tree, err := h.inherits.Tree(c.Request.Context())
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)
		return
	}

	roleIDStr := strings.TrimSpace(c.Query("roleId"))
	if roleIDStr == "" {
		res.OkWithData(tree, c)
		return
	}

	roleID, err := strconv.Atoi(roleIDStr)
	if err != nil || roleID <= 0 {
		res.FailWithMsg("角色ID非法", c)
		return
	}

	filtered, err := h.inherits.FilterTree(c.Request.Context(), tree, uint(roleID))
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)
		return
	}

	res.OkWithData(filtered, c)
}
