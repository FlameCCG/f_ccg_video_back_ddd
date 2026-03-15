package chat

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// DeleteConversationReq 删除会话请求参数.
type DeleteConversationReq struct {
	PeerID uint `json:"peerId" binding:"required,min=1" label:"对方用户ID"`
}

// DeleteConversationView 删除会话（仅对自己隐藏，幂等）.
func (h *Handler) DeleteConversationView(c *gin.Context) {
	req := middlewares.GetBound[DeleteConversationReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.service.DeleteConversation(c.Request.Context(), uid, req.PeerID); err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithMsg("删除成功", c)
}
