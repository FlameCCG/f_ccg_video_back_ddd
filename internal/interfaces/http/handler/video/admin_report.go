package video

import (
	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// AdminReportListReq 按举报对象种类、处理状态与分页查询举报；对象种类不是视频的审核状态。
type AdminReportListReq struct {
	Type     string            `form:"type" binding:"required,oneof=video danmu"`
	Status   enum.ReportStatus `form:"status" binding:"omitempty"`
	Page     int               `form:"page" binding:"omitempty,min=1"`
	PageSize int               `form:"pageSize" binding:"omitempty,min=1,max=100"`
}

// AdminReportListView 管理员举报记录列表.
func (h *AdminHandler) AdminReportListView(c *gin.Context) {
	req := middlewares.GetBound[AdminReportListReq](c)
	ctx := c.Request.Context()

	if err := h.reports.ValidateReportListStatus(req.Status); err != nil {
		res.FailWithMsg("status 非法", c)

		return
	}

	switch req.Type {
	case "video":
		list, total, err := h.reports.AdminVideoReports(ctx, req.Page, req.PageSize, int(req.Status))
		if err != nil {
			res.FailWithMsg(clienterror.Message(err), c)

			return
		}

		res.OkWithList(list, total, c)
	case "danmu":
		list, total, err := h.reports.AdminDanmuReports(ctx, req.Page, req.PageSize, int(req.Status))
		if err != nil {
			res.FailWithMsg(clienterror.Message(err), c)

			return
		}

		res.OkWithList(list, total, c)
	default:
		res.FailWithMsg("type 非法", c)
	}
}

// AdminReportHandleReq 表达同类举报的批量处理结论和备注，处理人从认证上下文取得。
type AdminReportHandleReq struct {
	Type       string            `json:"type" binding:"required,oneof=video danmu"`
	ReportIDs  []uint            `json:"reportIds" binding:"required,min=1,dive,gt=0"`
	Status     enum.ReportStatus `json:"status" binding:"required"`
	HandleNote string            `json:"handleNote" binding:"omitempty,max=500"`
}

// AdminReportHandleView 管理员处理举报记录.
func (h *AdminHandler) AdminReportHandleView(c *gin.Context) {
	req := middlewares.GetBound[AdminReportHandleReq](c)

	if req.Status != enum.ReportStatusResolved && req.Status != enum.ReportStatusRejected {
		res.FailWithMsg("status 非法", c)

		return
	}

	handlerID, _ := middlewares.GetUID(c)

	var handleType domain.ReportHandleType

	switch req.Type {
	case "video":
		handleType = domain.ReportHandleTypeVideo
	case "danmu":
		handleType = domain.ReportHandleTypeDanmu
	default:
		res.FailWithMsg("type 非法", c)

		return
	}

	affected, err := h.reports.HandleReports(
		c.Request.Context(), handleType, req.ReportIDs, req.Status, req.HandleNote, handlerID,
	)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithData(gin.H{"handled": affected}, c)
}

// AdminDanmuListReq 按发送用户、用户名和内容关键字筛选管理端弹幕列表。
type AdminDanmuListReq struct {
	UserID   uint   `form:"userId" binding:"omitempty,gt=0"`
	Username string `form:"username"`
	Keyword  string `form:"keyword"`
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"pageSize" binding:"omitempty,min=1,max=100"`
}

// AdminDanmuListView 管理员弹幕列表.
func (h *AdminHandler) AdminDanmuListView(c *gin.Context) {
	req := middlewares.GetBound[AdminDanmuListReq](c)

	list, total, err := h.danmu.AdminList(c.Request.Context(), app.AdminDanmuListParams{
		UserID:   req.UserID,
		Username: req.Username,
		Keyword:  req.Keyword,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithList(list, total, c)
}

// AdminDeleteDanmuReq 指定管理端批量删除的弹幕 ID，由用例协调点赞和分P计数处理。
type AdminDeleteDanmuReq struct {
	DanmuIDs []uint `json:"danmuIds" binding:"required,min=1,dive,gt=0"`
}

// AdminDeleteDanmuView 管理员批量删除弹幕.
func (h *AdminHandler) AdminDeleteDanmuView(c *gin.Context) {
	req := middlewares.GetBound[AdminDeleteDanmuReq](c)

	deleted, err := h.danmu.AdminDelete(c.Request.Context(), req.DanmuIDs)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithData(map[string]int64{"deleted": deleted}, c)
}
