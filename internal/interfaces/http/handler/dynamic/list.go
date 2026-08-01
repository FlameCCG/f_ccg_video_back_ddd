package dynamic

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// DynamicListReq 作品流列表请求.
type DynamicListReq struct {
	UserID   uint `form:"userId" binding:"omitempty,gt=0" label:"指定用户ID"`
	Type     int  `form:"type" binding:"omitempty,oneof=0 1 2" label:"类型"`
	Page     int  `form:"page" label:"页码"`
	PageSize int  `form:"pageSize" label:"每页数量"`
}

// ListView 指定作者的作品流（视频 + 动态）.
func (h *Handler) ListView(c *gin.Context) {
	req := middlewares.GetBound[DynamicListReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	if req.UserID > 0 && uid > 0 {
		_ = h.reads.MarkLatestRead(c.Request.Context(), uid, req.UserID)
	}

	list, total, err := h.feed.ListFeed(c.Request.Context(), uid, req.UserID, req.Type, req.Page, req.PageSize)
	if err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithList(list, total, c)
}
