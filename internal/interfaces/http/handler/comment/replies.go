package comment

import (
	app "f_ccg_video_back_ddd/internal/application/comment"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// RepliesReq 以 RootID 查询一条根讨论下的回复分页，不是只查某条中间回复的直接子节点。
type RepliesReq struct {
	RootID   uint `form:"rootId" binding:"required" label:"根评论ID"`
	Page     int  `form:"page" label:"页码"`
	PageSize int  `form:"pageSize" label:"每页数量"`
}

// RepliesView 将根评论与分页参数传给应用查询，返回讨论回复列表。
func (h *Handler) RepliesView(c *gin.Context) {
	req := middlewares.GetBound[RepliesReq](c)

	var uid uint
	if v, ok := middlewares.GetUID(c); ok {
		uid = v
	}

	list, total, err := h.svc.Replies(c.Request.Context(), app.RepliesParams{
		RootID:   req.RootID,
		ViewerID: uid,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)

		return
	}

	res.OkWithList(list, total, c)
}
