package rbac

import (
	"strconv"
	"strings"

	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// CreateMenuReq 创建菜单请求。
type CreateMenuReq struct {
	Title     string `json:"title" binding:"required" label:"菜单标题"`
	TitleEn   string `json:"titleEn" label:"菜单英文标题"`
	TitleJa   string `json:"titleJa" label:"菜单日文标题"`
	Icon      string `json:"icon" label:"图标"`
	Path      string `json:"path" label:"路由路径"`
	Name      string `json:"name" label:"路由名称"`
	Component string `json:"component" label:"前端组件路径/标识"`
	KeepAlive bool   `json:"keepAlive" label:"是否缓存"`
	ParentID  *uint  `json:"parentId" label:"父菜单ID"`
	SortOrder uint   `json:"sortOrder" label:"排序"`
}

// UpdateMenuReq 更新菜单请求。
type UpdateMenuReq struct {
	ID        uint    `json:"id" binding:"required,gt=0" label:"菜单ID"`
	Title     *string `json:"title" label:"菜单标题"`
	TitleEn   *string `json:"titleEn" label:"菜单英文标题"`
	TitleJa   *string `json:"titleJa" label:"菜单日文标题"`
	Icon      *string `json:"icon" label:"图标"`
	Path      *string `json:"path" label:"路由路径"`
	Name      *string `json:"name" label:"路由名称"`
	Component *string `json:"component" label:"前端组件路径/标识"`
	KeepAlive *bool   `json:"keepAlive" label:"是否缓存"`
	ParentID  *uint   `json:"parentId" label:"父菜单ID"`
	SortOrder *uint   `json:"sortOrder" label:"排序"`
}

// AssignMenuReq 分配/移除角色菜单请求。
type AssignMenuReq struct {
	RoleID  uint   `json:"roleId" binding:"required" label:"角色ID"`
	MenuIDs []uint `json:"menuIds" binding:"required" label:"菜单ID列表"` // 支持单个和批量
}

// GetMenuListView 获取所有菜单（树结构）.
func (h *Handler) GetMenuListView(c *gin.Context) {
	menuTree, err := h.menus.Tree(c.Request.Context())
	if err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithData(menuTree, c)
}

// CreateMenuView 创建菜单.
func (h *Handler) CreateMenuView(c *gin.Context) {
	req := middlewares.GetBound[CreateMenuReq](c)

	if err := h.menus.Create(c.Request.Context(), rbacapp.CreateMenuParams{
		Title:     req.Title,
		TitleEn:   req.TitleEn,
		TitleJa:   req.TitleJa,
		Icon:      req.Icon,
		Path:      req.Path,
		Name:      req.Name,
		Component: req.Component,
		KeepAlive: req.KeepAlive,
		ParentID:  req.ParentID,
		SortOrder: req.SortOrder,
	}); err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithMsg("创建成功", c)
}

// UpdateMenuView 更新菜单.
func (h *Handler) UpdateMenuView(c *gin.Context) {
	req := middlewares.GetBound[UpdateMenuReq](c)

	if err := h.menus.Update(c.Request.Context(), rbacapp.UpdateMenuParams{
		ID:        req.ID,
		Title:     req.Title,
		TitleEn:   req.TitleEn,
		TitleJa:   req.TitleJa,
		Icon:      req.Icon,
		Path:      req.Path,
		Name:      req.Name,
		Component: req.Component,
		KeepAlive: req.KeepAlive,
		ParentID:  req.ParentID,
		SortOrder: req.SortOrder,
	}); err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithMsg("更新成功", c)
}

// DeleteMenuView 删除菜单.
// Query: id
func (h *Handler) DeleteMenuView(c *gin.Context) {
	id := strings.TrimSpace(c.Query("id"))
	if id == "" {
		res.FailWithMsg("菜单ID不能为空", c)
		return
	}

	menuID, ok := parsePositiveID(id)
	if !ok {
		res.FailWithMsg("菜单不存在", c)
		return
	}

	if err := h.menus.Delete(c.Request.Context(), menuID); err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithMsg("删除成功", c)
}

// AssignMenuView 分配角色菜单（整体替换直连菜单）.
func (h *Handler) AssignMenuView(c *gin.Context) {
	req := middlewares.GetBound[AssignMenuReq](c)

	if err := h.menus.AssignRoleMenus(c.Request.Context(), req.RoleID, req.MenuIDs); err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithMsg("更新成功", c)
}

// RemoveRoleMenuView 移除角色菜单.
func (h *Handler) RemoveRoleMenuView(c *gin.Context) {
	req := middlewares.GetBound[AssignMenuReq](c)

	if err := h.menus.RemoveRoleMenus(c.Request.Context(), req.RoleID, req.MenuIDs); err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithMsg("移除成功", c)
}

// RoleMenusView 根据角色获取菜单列表.
// Query: roleId
func (h *Handler) RoleMenusView(c *gin.Context) {
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

	resp, err := h.menus.RoleMenus(c.Request.Context(), uint(roleID))
	if err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithData(resp, c)
}

// GetUserMenusView 获取当前用户菜单.
func (h *Handler) GetUserMenusView(c *gin.Context) {
	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	menuTree, err := h.menus.UserMenus(c.Request.Context(), uid)
	if err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithData(menuTree, c)
}
