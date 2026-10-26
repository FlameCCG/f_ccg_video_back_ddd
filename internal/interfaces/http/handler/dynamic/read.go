package dynamic

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// ReadReq 标记已读并拉取作品流请求.
type ReadReq struct {
	AuthorID uint `json:"authorId" binding:"required,gt=0" label:"作者ID"`
	Page     int  `json:"page" label:"页码"`
	PageSize int  `json:"pageSize" label:"每页数量"`
}

// ReadView 把作者最新作品标记为已读，并返回该作者的作品流.
func (h *Handler) ReadView(c *gin.Context) {
	req := middlewares.GetBound[ReadReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	list, total, err := h.reads.ReadLatestAndList(c.Request.Context(), uid, req.AuthorID, req.Page, req.PageSize)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)
		return
	}

	res.OkWithList(list, total, c)
}
