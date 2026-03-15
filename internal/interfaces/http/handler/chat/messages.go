package chat

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// MessageListReq 消息列表请求参数.
type MessageListReq struct {
	PeerID uint   `form:"peerId" binding:"required,min=1" label:"对方用户ID"`
	Before string `form:"before" binding:"omitempty,len=24" label:"游标"`
}

// MessageListView 获取会话消息（固定返回 10 条，按 before 游标向前翻页）.
func (h *Handler) MessageListView(c *gin.Context) {
	req := middlewares.GetBound[MessageListReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	data, err := h.service.ListMessages(c.Request.Context(), uid, req.PeerID, req.Before)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithData(data, c)
}
