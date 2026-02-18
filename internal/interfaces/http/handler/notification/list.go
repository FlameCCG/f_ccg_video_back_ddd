package notification

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// ListNotificationReq is an exported API type.
type ListNotificationReq struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"pageSize" binding:"omitempty,min=1,max=100"`
	Category string `form:"category" binding:"omitempty,oneof=all reply like at system"`
}

// List handles the corresponding HTTP request.
func (h *Handler) List(c *gin.Context) {
	req := middlewares.GetBound[ListNotificationReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	list, total, err := h.svc.List(c.Request.Context(), int(uid), req.Page, req.PageSize, req.Category)
	if err != nil {
		res.FailWithMsg("获取通知失败", c)

		return
	}

	res.OkWithList(list, total, c)
}

// Counts 返回各类型未读数量.
func (h *Handler) Counts(c *gin.Context) {
	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	data, err := h.svc.CountSummary(c.Request.Context(), int(uid))
	if err != nil {
		res.FailWithMsg("获取数量失败", c)

		return
	}

	res.OkWithData(data, c)
}
