package video

import (
	app "f_ccg_video_back_ddd/internal/application/video"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// CreatePartitionReq is an exported API type.
type CreatePartitionReq struct {
	Name          string `json:"name" binding:"required,max=50"`
	Icon          string `json:"icon" binding:"required"`
	SortOrder     int    `json:"sortOrder" binding:"omitempty"`
	IsActive      bool   `json:"isActive"`
	IsSubmittable bool   `json:"isSubmittable"`
}

// UpdatePartitionReq is an exported API type.
type UpdatePartitionReq struct {
	ID            uint   `json:"id" binding:"required,gt=0"`
	Name          string `json:"name" binding:"required,max=50"`
	Icon          string `json:"icon" binding:"omitempty,max=65535"`
	SortOrder     int    `json:"sortOrder" binding:"omitempty"`
	IsActive      bool   `json:"isActive"`
	IsSubmittable bool   `json:"isSubmittable"`
}

// PartitionListReq is an exported API type.
type PartitionListReq struct {
	Page     int `form:"page" binding:"omitempty,min=1"`
	PageSize int `form:"pageSize" binding:"omitempty,min=1,max=100"`
}

// DeletePartitionReq is an exported API type.
type DeletePartitionReq struct {
	ID uint `json:"id" binding:"required,gt=0"`
}

// CreatePartitionView handles the corresponding HTTP request.
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
		res.FailWithMsg("创建失败", c)

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

// UpdatePartitionView handles the corresponding HTTP request.
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
		res.FailWithMsg("更新失败", c)

		return
	}

	res.OkWithMsg("更新成功", c)
}

// DeletePartitionView handles the corresponding HTTP request.
func (h *AdminHandler) DeletePartitionView(c *gin.Context) {
	req := middlewares.GetBound[DeletePartitionReq](c)
	if err := h.partition.Delete(c.Request.Context(), req.ID); err != nil {
		res.FailWithMsg("删除失败", c)

		return
	}

	res.OkWithMsg("删除成功", c)
}

// ListPartitionView handles the corresponding HTTP request.
func (h *AdminHandler) ListPartitionView(c *gin.Context) {
	req := middlewares.GetBound[PartitionListReq](c)

	list, total, err := h.partition.List(c.Request.Context(), req.Page, req.PageSize)
	if err != nil {
		res.FailWithMsg("获取失败", c)

		return
	}

	res.OkWithList(list, total, c)
}
