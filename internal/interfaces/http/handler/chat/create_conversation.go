package chat

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// CreateConversationReq 创建会话请求参数.
type CreateConversationReq struct {
	PeerID uint `json:"peerId" binding:"required,min=1" label:"对方用户ID"`
}

// CreateConversationView 创建/恢复会话（幂等）.
func (h *Handler) CreateConversationView(c *gin.Context) {
	req := middlewares.GetBound[CreateConversationReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	item, err := h.service.CreateConversation(c, uid, req.PeerID)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithData(item, c)
}
