package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// LoginIPRecordListReq 请求/响应结构体.
type LoginIPRecordListReq struct {
	Page     int `form:"page" binding:"omitempty,min=1" label:"页码"`
	PageSize int `form:"pageSize" binding:"omitempty,min=1,max=100" label:"每页数量"`
}

// LoginIPRecordListView 获取当前用户登录IP记录（需登录）.
func (h *Handler) LoginIPRecordListView(c *gin.Context) {
	req := middlewares.GetBound[LoginIPRecordListReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	list, total, err := h.loginRecords.List(c.Request.Context(), uid, req.Page, req.PageSize)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithList(list, total, c)
}
