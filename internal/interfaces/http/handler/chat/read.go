package chat

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// MarkReadReq 标记已读请求参数.
type MarkReadReq struct {
	PeerID uint `json:"peerId" binding:"required,min=1" label:"对方用户ID"`
}

// MarkReadView 标记与某用户的私信已读（红点清零）.
func (h *Handler) MarkReadView(c *gin.Context) {
	req := middlewares.GetBound[MarkReadReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.service.MarkRead(c.Request.Context(), uid, req.PeerID); err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithMsg("标记成功", c)
}
