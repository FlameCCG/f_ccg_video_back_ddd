package comment

import (
	app "f_ccg_video_back_ddd/internal/application/comment"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// CommentRepliesReq is an exported API type.
type CommentRepliesReq struct {
	RootID   uint `form:"rootId" binding:"required" label:"根评论ID"`
	Page     int  `form:"page" label:"页码"`
	PageSize int  `form:"pageSize" label:"每页数量"`
}

// RepliesView handles the corresponding HTTP request.
func (h *Handler) RepliesView(c *gin.Context) {
	req := middlewares.GetBound[CommentRepliesReq](c)

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
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithList(list, total, c)
}
