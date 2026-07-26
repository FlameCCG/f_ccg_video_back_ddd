package dynamic

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// DynamicFollowListReq 关注列表请求.
type DynamicFollowListReq struct {
	Page     int `form:"page" label:"页码"`
	PageSize int `form:"pageSize" label:"每页数量"`
}

// FollowListView 当前用户的关注列表（含最新作品与未读标记）.
func (h *Handler) FollowListView(c *gin.Context) {
	req := middlewares.GetBound[DynamicFollowListReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	list, total, err := h.follows.ListFollowUsers(c.Request.Context(), uid, req.Page, req.PageSize)
	if err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithList(list, total, c)
}
