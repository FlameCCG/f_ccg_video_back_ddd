package rbac

import (
	"errors"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/res"
	"strings"

	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"
	domain "f_ccg_video_back_ddd/internal/domain/rbac"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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

// MoveMenuReq 菜单同层拖拽排序请求。
type MoveMenuReq struct {
	ID       uint   `json:"id" binding:"required,gt=0" label:"菜单ID"`
	TargetID uint   `json:"targetId" binding:"required,gt=0" label:"目标菜单ID"`
	Position string `json:"position" binding:"required,oneof=before after" label:"放置位置"`
}

// GenerateMenuReq AI 生成菜单字段请求。
type GenerateMenuReq struct {
	Title    string `json:"title" binding:"required" label:"中文标题"`
	ParentID *uint  `json:"parentId" label:"父菜单ID"`
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
		res.FailWithCause(err, clienterror.Message(err), c)
		return
	}

	res.OkWithData(menuTree, c)
}

// CreateMenuView 创建菜单.
func (h *Handler) CreateMenuView(c *gin.Context) {
	req := middlewares.GetBound[CreateMenuReq](c)

	title := strings.TrimSpace(req.Title)
	if title == "" {
		res.FailWithMsg("菜单标题不能为空", c)

		return
	}

	creatorID, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("未获取到创建者信息", c)

		return
	}

	if err := h.menus.Create(c.Request.Context(), rbacapp.CreateMenuParams{
		Title:     title,
		TitleEn:   strings.TrimSpace(req.TitleEn),
		TitleJa:   strings.TrimSpace(req.TitleJa),
		Icon:      strings.TrimSpace(req.Icon),
		Path:      strings.TrimSpace(req.Path),
		Name:      strings.TrimSpace(req.Name),
		Component: strings.TrimSpace(req.Component),
		KeepAlive: req.KeepAlive,
		ParentID:  req.ParentID,
		SortOrder: req.SortOrder,
		CreatorID: creatorID,
	}); err != nil {
		h.respondMenuMutationError(
			c,
			"create",
			err,
			domain.ErrCreateFailed,
			zap.Uint("creatorId", creatorID),
			optionalMenuIDField("parentMenuId", req.ParentID),
		)

		return
	}

	res.OkWithMsg("创建成功", c)
}

// UpdateMenuView 更新菜单.
func (h *Handler) UpdateMenuView(c *gin.Context) {
	req := middlewares.GetBound[UpdateMenuReq](c)
	if req.Title == nil && req.TitleEn == nil && req.TitleJa == nil && req.Icon == nil &&
		req.Path == nil && req.Name == nil && req.Component == nil && req.KeepAlive == nil &&
		req.ParentID == nil && req.SortOrder == nil {
		res.FailWithMsg("没有需要更新的字段", c)

		return
	}

	if err := h.menus.Update(c.Request.Context(), rbacapp.UpdateMenuParams{
		ID:             req.ID,
		Title:          req.Title,
		TitleEn:        req.TitleEn,
		TitleJa:        req.TitleJa,
		Icon:           req.Icon,
		Path:           req.Path,
		Name:           req.Name,
		Component:      req.Component,
		KeepAlive:      req.KeepAlive,
		ParentID:       req.ParentID,
		ParentProvided: req.ParentID != nil,
		SortOrder:      req.SortOrder,
	}); err != nil {
		h.respondMenuMutationError(
			c,
			"update",
			err,
			domain.ErrUpdateFailed,
			zap.Uint("menuId", req.ID),
			optionalMenuIDField("parentMenuId", req.ParentID),
		)

		return
	}

	res.OkWithMsg("更新成功", c)
}

// MoveMenuView 处理菜单同层拖拽排序。
func (h *Handler) MoveMenuView(c *gin.Context) {
	req := middlewares.GetBound[MoveMenuReq](c)
	if err := h.menus.Move(c.Request.Context(), req.ID, req.TargetID, req.Position); err != nil {
		h.respondMenuMutationError(
			c,
			"move",
			err,
			domain.ErrSortFailed,
			zap.Uint("menuId", req.ID),
			zap.Uint("targetMenuId", req.TargetID),
		)

		return
	}

	res.OkWithMsg("排序成功", c)
}

// GenerateMenuView 根据中文标题、父菜单和 menus 表完整数据生成其余字段。
func (h *Handler) GenerateMenuView(c *gin.Context) {
	req := middlewares.GetBound[GenerateMenuReq](c)

	suggestion, err := h.menus.Generate(c.Request.Context(), req.Title, req.ParentID)
	if err != nil {
		h.respondMenuGenerationError(c, err, req.ParentID)

		return
	}

	res.OkWithData(suggestion, c)
}

// DeleteMenuView 删除菜单.
// Query: id.
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
		res.FailWithCause(err, clienterror.Message(err), c)
		return
	}

	res.OkWithMsg("删除成功", c)
}

// AssignMenuView 分配角色菜单（整体替换直连菜单）.
func (h *Handler) AssignMenuView(c *gin.Context) {
	req := middlewares.GetBound[AssignMenuReq](c)

	if err := h.menus.AssignRoleMenus(c.Request.Context(), req.RoleID, req.MenuIDs); err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)
		return
	}

	res.OkWithMsg("更新成功", c)
}

// RemoveRoleMenuView 移除角色菜单.
func (h *Handler) RemoveRoleMenuView(c *gin.Context) {
	req := middlewares.GetBound[AssignMenuReq](c)

	if err := h.menus.RemoveRoleMenus(c.Request.Context(), req.RoleID, req.MenuIDs); err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)
		return
	}

	res.OkWithMsg("移除成功", c)
}

// RoleMenusView 根据角色获取菜单列表.
// Query: roleId.
func (h *Handler) RoleMenusView(c *gin.Context) { serveIDLookup(c, "roleId", h.menus.RoleMenus) }

// GetUserMenusView 获取当前用户菜单.
func (h *Handler) GetUserMenusView(c *gin.Context) {
	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	menuTree, err := h.menus.UserMenus(c.Request.Context(), uid)
	if err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)
		return
	}

	res.OkWithData(menuTree, c)
}

func (h *Handler) respondMenuMutationError(
	c *gin.Context,
	operation string,
	err error,
	fallback error,
	_ ...zap.Field,
) {
	for _, businessErr := range []error{
		domain.ErrNoUpdatableFields,
		domain.ErrMenuNotFound,
		domain.ErrMenuExists,
		domain.ErrMenuTitleEmpty,
		domain.ErrParentMenuNotFound,
		domain.ErrMenuParentCycle,
		domain.ErrMenuCreatorRoleMissing,
		domain.ErrMenuMoveAcrossLevels,
		domain.ErrMenuMovePosition,
		domain.ErrMenuIconUnsafe,
		domain.ErrMenuIconTooLong,
	} {
		if errors.Is(err, businessErr) {
			res.FailWithCause(err, clienterror.Message(businessErr), c)

			return
		}
	}

	res.FailWithCause(apperr.Wrap(operation, fallback, err), clienterror.Message(fallback), c)
}

func (h *Handler) respondMenuGenerationError(c *gin.Context, err error, _ *uint) {
	for _, businessErr := range []error{
		domain.ErrMenuGenerationTitleEmpty,
		domain.ErrParentMenuNotFound,
	} {
		if errors.Is(err, businessErr) {
			res.FailWithCause(err, clienterror.Message(businessErr), c)

			return
		}
	}

	publicErr := err
	for _, fallback := range []error{
		domain.ErrReadParentMenuFailed,
		domain.ErrReadMenuDataFailed,
		domain.ErrPrepareMenuDataFailed,
		domain.ErrMenuGeneratorUnavailable,
	} {
		if errors.Is(err, fallback) {
			publicErr = fallback

			break
		}
	}

	res.FailWithCause(err, clienterror.Message(publicErr), c)
}

func optionalMenuIDField(key string, value *uint) zap.Field {
	if value == nil {
		return zap.Skip()
	}

	return zap.Uint(key, *value)
}
