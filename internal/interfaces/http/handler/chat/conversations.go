package chat

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// ConversationListReq 会话列表请求参数.
type ConversationListReq struct {
	Page     int `form:"page" binding:"omitempty,min=1" label:"页码"`
	PageSize int `form:"pageSize" binding:"omitempty,min=1,max=50" label:"每页数量"`
}

// ConversationListView 会话列表（pageSize 最大 50，service 层二次 clamp）.
func (h *Handler) ConversationListView(c *gin.Context) {
	req := middlewares.GetBound[ConversationListReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	list, total, err := h.service.ListConversations(c.Request.Context(), uid, req.Page, req.PageSize)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithList(list, total, c)
}
