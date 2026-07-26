package dynamic

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// DynamicCountsResp 关注未读数响应.
type DynamicCountsResp struct {
	UnreadCount int64 `json:"unreadCount"`
}

// CountsView 关注对象的未读作品人数（角标）.
func (h *Handler) CountsView(c *gin.Context) {
	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	count, err := h.follows.UnreadFollowCount(c.Request.Context(), uid)
	if err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithData(DynamicCountsResp{UnreadCount: count}, c)
}
