package contentops

import (
	"errors"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	app "f_ccg_video_back_ddd/internal/application/contentops"

	"github.com/gin-gonic/gin"
)

// BannerListReq 请求/响应结构体.
type BannerListReq struct {
	Type        int   `form:"type" binding:"omitempty,oneof=1 2 3"`
	PartitionID *uint `form:"partitionId"`
}

// AdminBannerListReq 请求/响应结构体.
type AdminBannerListReq struct {
	Page        int   `form:"page" binding:"omitempty,min=1"`
	PageSize    int   `form:"pageSize" binding:"omitempty,min=1,max=100"`
	Type        int   `form:"type" binding:"omitempty,oneof=1 2 3"`
	PartitionID *uint `form:"partitionId"`
	Show        *bool `form:"show"`
}

// AdminCreateBannerReq 请求/响应结构体.
type AdminCreateBannerReq struct {
	Cover       string `json:"cover" binding:"required,max=255" label:"图片链接"`
	Href        string `json:"href" binding:"omitempty,max=255" label:"跳转链接"`
	Show        *bool  `json:"show" label:"是否显示"`
	Type        int    `json:"type" binding:"omitempty,oneof=1 2 3" label:"轮播图类型"`
	PartitionID *uint  `json:"partitionId" label:"分区ID"`
}

// AdminUpdateBannerReq 请求/响应结构体.
type AdminUpdateBannerReq struct {
	ID          uint    `json:"id" binding:"required,gt=0" label:"轮播图ID"`
	Cover       *string `json:"cover" binding:"omitempty,max=255" label:"图片链接"`
	Href        *string `json:"href" binding:"omitempty,max=255" label:"跳转链接"`
	Show        *bool   `json:"show" label:"是否显示"`
	Type        *int    `json:"type" binding:"omitempty,oneof=1 2 3" label:"轮播图类型"`
	PartitionID *uint   `json:"partitionId" label:"分区ID"`
}

// AdminDeleteBannerReq 请求/响应结构体.
type AdminDeleteBannerReq struct {
	BannerIDs []uint `json:"bannerIds" binding:"required,min=1,dive,gt=0" label:"轮播图ID列表"`
}

// ListView 获取可展示的轮播图列表.
func (h *Handler) ListView(c *gin.Context) {
	req := middlewares.GetBound[BannerListReq](c)

	list, err := h.banners.ListVisible(c.Request.Context(), app.VisibleBannerParams{
		Type:        req.Type,
		PartitionID: req.PartitionID,
	})
	if err != nil {
		if errors.Is(err, app.ErrBannerPartitionFilterUnsupported) {
			res.FailWithCause(err, "只有类型1支持按分区筛选", c)

			return
		}

		res.FailWithCause(err, "获取失败", c)

		return
	}

	res.OkWithData(list, c)
}

// AdminListView 后台轮播图列表.
func (h *Handler) AdminListView(c *gin.Context) {
	req := middlewares.GetBound[AdminBannerListReq](c)

	list, total, err := h.banners.ListAdmin(c.Request.Context(), app.AdminBannerListParams{
		Page:        req.Page,
		PageSize:    req.PageSize,
		Type:        req.Type,
		PartitionID: req.PartitionID,
		Show:        req.Show,
	})
	if err != nil {
		if errors.Is(err, app.ErrBannerPartitionFilterUnsupported) {
			res.FailWithCause(err, "只有类型1支持按分区筛选", c)

			return
		}

		res.FailWithCause(err, "获取失败", c)

		return
	}

	res.OkWithList(list, total, c)
}

// AdminCreateView 创建轮播图.
func (h *Handler) AdminCreateView(c *gin.Context) {
	req := middlewares.GetBound[AdminCreateBannerReq](c)

	banner, err := h.banners.Create(c.Request.Context(), app.CreateBannerParams{
		Cover:       req.Cover,
		Href:        req.Href,
		Show:        req.Show,
		Type:        req.Type,
		PartitionID: req.PartitionID,
	})
	if err != nil {
		// 分区裁定失败回显原始文案，落库失败回显固定文案（与原实现一致）。
		var partitionErr *app.PartitionResolveError
		if errors.As(err, &partitionErr) {
			res.FailWithCause(partitionErr, clienterror.Message(partitionErr), c)

			return
		}

		res.FailWithCause(err, "创建失败", c)

		return
	}

	res.OkWithData(banner, c)
}

// AdminUpdateView 更新轮播图.
func (h *Handler) AdminUpdateView(c *gin.Context) {
	req := middlewares.GetBound[AdminUpdateBannerReq](c)

	err := h.banners.Update(c.Request.Context(), app.UpdateBannerParams{
		ID:          req.ID,
		Cover:       req.Cover,
		Href:        req.Href,
		Show:        req.Show,
		Type:        req.Type,
		PartitionID: req.PartitionID,
	})
	if err != nil {
		if errors.Is(err, app.ErrBannerNotFound) {
			res.FailWithCause(err, "轮播图不存在", c)

			return
		}

		res.FailWithCause(err, clienterror.Message(err), c)

		return
	}

	res.OkWithMsg("更新成功", c)
}

// AdminDeleteView 批量删除轮播图.
func (h *Handler) AdminDeleteView(c *gin.Context) {
	req := middlewares.GetBound[AdminDeleteBannerReq](c)

	if err := h.banners.Delete(c.Request.Context(), req.BannerIDs); err != nil {
		res.FailWithCause(err, "删除失败", c)

		return
	}

	res.OkWithMsg("删除成功", c)
}
