package video

import (
	app "f_ccg_video_back_ddd/internal/application/video"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// CreatePartitionReq 定义分区名称、展示、排序和启用/可投稿策略；两种开关分别表示可用与接收投稿。
type CreatePartitionReq struct {
	Name          string `json:"name" binding:"required,max=50"`
	Icon          string `json:"icon" binding:"required"`
	SortOrder     int    `json:"sortOrder" binding:"omitempty"`
	IsActive      bool   `json:"isActive"`
	IsSubmittable bool   `json:"isSubmittable"`
}

// UpdatePartitionReq 指定分区 ID 并提供完整更新字段，非指针布尔值的零值会表达关闭。
type UpdatePartitionReq struct {
	ID            uint   `json:"id" binding:"required,gt=0"`
	Name          string `json:"name" binding:"required,max=50"`
	Icon          string `json:"icon" binding:"omitempty,max=65535"`
	SortOrder     int    `json:"sortOrder" binding:"omitempty"`
	IsActive      bool   `json:"isActive"`
	IsSubmittable bool   `json:"isSubmittable"`
}

// PartitionListReq 限定管理端分区查询的页码和每页条数。
type PartitionListReq struct {
	Page     int `form:"page" binding:"omitempty,min=1"`
	PageSize int `form:"pageSize" binding:"omitempty,min=1,max=100"`
}

// DeletePartitionReq 指定待删除分区，分区引用等业务检查由应用用例执行。
type DeletePartitionReq struct {
	ID uint `json:"id" binding:"required,gt=0"`
}

// CreatePartitionView 将分区字段翻译为应用创建参数，返回创建结果。
func (h *AdminHandler) CreatePartitionView(c *gin.Context) {
	req := middlewares.GetBound[CreatePartitionReq](c)

	p := domain.Partition{
		Name:          req.Name,
		Icon:          req.Icon,
		SortOrder:     req.SortOrder,
		IsActive:      req.IsActive,
		IsSubmittable: req.IsSubmittable,
	}
	if err := h.partition.Create(c.Request.Context(), &p); err != nil {
		res.FailWithCause(err, "创建失败", c)

		return
	}

	res.OkWithData(app.PartitionItem{
		ID:            p.ID,
		Name:          p.Name,
		Icon:          p.Icon,
		SortOrder:     p.SortOrder,
		IsActive:      p.IsActive,
		IsSubmittable: p.IsSubmittable,
	}, c)
}

// UpdatePartitionView 把分区 ID 和更新参数交给应用服务，接口层不直接更新数据库列。
func (h *AdminHandler) UpdatePartitionView(c *gin.Context) {
	req := middlewares.GetBound[UpdatePartitionReq](c)

	p := domain.Partition{
		ID:            req.ID,
		Name:          req.Name,
		Icon:          req.Icon,
		SortOrder:     req.SortOrder,
		IsActive:      req.IsActive,
		IsSubmittable: req.IsSubmittable,
	}
	if err := h.partition.Update(c.Request.Context(), &p); err != nil {
		res.FailWithCause(err, "更新失败", c)

		return
	}

	res.OkWithMsg("更新成功", c)
}

// DeletePartitionView 调用分区删除用例并映射业务错误。
func (h *AdminHandler) DeletePartitionView(c *gin.Context) {
	req := middlewares.GetBound[DeletePartitionReq](c)
	if err := h.partition.Delete(c.Request.Context(), req.ID); err != nil {
		res.FailWithCause(err, "删除失败", c)

		return
	}

	res.OkWithMsg("删除成功", c)
}

// ListPartitionView 调用分区分页查询并返回列表和总条数。
func (h *AdminHandler) ListPartitionView(c *gin.Context) {
	req := middlewares.GetBound[PartitionListReq](c)

	list, total, err := h.partition.List(c.Request.Context(), req.Page, req.PageSize)
	if err != nil {
		res.FailWithCause(err, "获取失败", c)

		return
	}

	res.OkWithList(list, total, c)
}
