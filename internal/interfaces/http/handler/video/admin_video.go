package video

import (
	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// AdminVideoDetailReq 指定管理端详情目标；管理查询可查看非公开稿，不能复用为公开详情入口。
type AdminVideoDetailReq struct {
	VideoID uint `form:"videoId" binding:"required,gt=0"`
}

// AdminDetailView 管理员获取视频详情.
func (h *AdminHandler) AdminDetailView(c *gin.Context) {
	req := middlewares.GetBound[AdminVideoDetailReq](c)

	detail, err := h.detail.GetVideoDetailAdmin(c.Request.Context(), req.VideoID)
	if err != nil {
		if isRecordNotFound(err) {
			res.FailWithMsg("视频不存在", c)

			return
		}

		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithData(detail, c)
}

// AdminVideoListReq 组织管理端筛选与排序；IsScheduled 为 nil 表示不按定时标记过滤，指向 false 表示只查非定时。
type AdminVideoListReq struct {
	IsScheduled *bool  `form:"isScheduled"`
	UserID      uint   `form:"userId" binding:"omitempty,gt=0"`
	Keyword     string `form:"keyword"`
	Status      int    `form:"status" binding:"omitempty,oneof=1 2 3 4 5"` // 1已发布 2私密 3已删除 4待审核 5定时待发布
	PartitionID uint   `form:"partitionId" binding:"omitempty,gt=0"`
	Sort        string `form:"sort" binding:"omitempty,oneof=latest oldest views likes duration"`
	Page        int    `form:"page" binding:"omitempty,min=1"`
	PageSize    int    `form:"pageSize" binding:"omitempty,min=1,max=100"`
}

// AdminListView 管理员视频列表.
func (h *AdminHandler) AdminListView(c *gin.Context) {
	req := middlewares.GetBound[AdminVideoListReq](c)

	list, total, err := h.admin.List(c.Request.Context(), app.AdminVideoListParams{
		IsScheduled: req.IsScheduled,
		UserID:      req.UserID,
		Keyword:     req.Keyword,
		Status:      req.Status,
		PartitionID: req.PartitionID,
		Sort:        req.Sort,
		Page:        req.Page,
		PageSize:    req.PageSize,
	})
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithList(list, total, c)
}

// AdminVideoReviewReq 表达批量审核的视频集合与目标状态，领域规则限制允许的审核结果。
type AdminVideoReviewReq struct {
	VideoIDs []uint `json:"videoIds" binding:"required,min=1,dive,gt=0"`
	Status   int    `json:"status" binding:"required,oneof=1 2 3"`
}

// AdminReviewView 管理员审核视频.
func (h *AdminHandler) AdminReviewView(c *gin.Context) {
	req := middlewares.GetBound[AdminVideoReviewReq](c)

	affected, err := h.admin.Review(c.Request.Context(), app.AdminVideoReviewParams{
		VideoIDs: req.VideoIDs,
		Status:   enum.VideoStatus(req.Status),
	})
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithData(map[string]int64{"updated": affected}, c)
}

// AdminRestoreVideoReq 表达管理端恢复已删除稿件的目标集合，与作者改稿权限分开。
type AdminRestoreVideoReq struct {
	VideoIDs []uint `json:"videoIds" binding:"required,min=1,dive,gt=0"`
}

// AdminRestoreVideoView 管理员批量恢复上架视频.
func (h *AdminHandler) AdminRestoreVideoView(c *gin.Context) {
	req := middlewares.GetBound[AdminRestoreVideoReq](c)

	affected, err := h.admin.Restore(c.Request.Context(), req.VideoIDs)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithData(gin.H{"restored": affected}, c)
}

// AdminDeleteVideoReq 表达批量删除意图；HardDelete 区分状态删除与物理清理，存储清理还需核对共享引用。
type AdminDeleteVideoReq struct {
	VideoIDs   []uint `json:"videoIds" binding:"required,min=1,dive,gt=0"`
	HardDelete bool   `json:"hardDelete"`
}

// AdminDeleteVideoView 管理员删除视频（软删/物理删）.
func (h *AdminHandler) AdminDeleteVideoView(c *gin.Context) {
	req := middlewares.GetBound[AdminDeleteVideoReq](c)

	result, err := h.admin.Delete(c.Request.Context(), req.VideoIDs, req.HardDelete)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithData(result, c)
}
