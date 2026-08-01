package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// ExpRecordListReq 请求/响应结构体.
type ExpRecordListReq struct {
	Page     int `form:"page" binding:"omitempty,min=1" label:"页码"`
	PageSize int `form:"pageSize" binding:"omitempty,min=1,max=100" label:"每页数量"`
}

// ExpRecordListView 获取当前用户经验记录（需登录）.
func (h *Handler) ExpRecordListView(c *gin.Context) {
	req := middlewares.GetBound[ExpRecordListReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	list, total, err := h.expRecords.List(c.Request.Context(), uid, req.Page, req.PageSize)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithList(list, total, c)
}
