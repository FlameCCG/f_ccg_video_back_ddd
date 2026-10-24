package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// CoinRecordListReq 请求/响应结构体.
type CoinRecordListReq struct {
	Page     int `form:"page" binding:"omitempty,min=1" label:"页码"`
	PageSize int `form:"pageSize" binding:"omitempty,min=1,max=100" label:"每页数量"`
}

// CoinRecordListView 获取当前用户硬币记录（需登录）.
func (h *Handler) CoinRecordListView(c *gin.Context) {
	req := middlewares.GetBound[CoinRecordListReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	list, total, err := h.coinRecords.List(c.Request.Context(), uid, req.Page, req.PageSize)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithList(list, total, c)
}
