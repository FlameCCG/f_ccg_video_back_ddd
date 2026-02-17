package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"

	"go.uber.org/zap"
)

// MentionSuggestReq 请求/响应结构体.
type MentionSuggestReq struct {
	Keyword  string `form:"keyword"`
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"pageSize" binding:"omitempty,min=1,max=50"`
}

// MentionSuggestView 提供评论 @ 联想，按好友优先返回关注列表.
func (h *Handler) MentionSuggestView(c *gin.Context) {
	req := middlewares.GetBound[MentionSuggestReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	list, total, err := h.activity.SuggestMentions(c.Request.Context(), uid, req.Keyword, req.Page, req.PageSize)
	if err != nil {
		h.log().Error("查询提及联想用户失败", zap.Error(err))
		res.FailWithMsg("查询失败", c)

		return
	}

	res.OkWithList(list, total, c)
}
