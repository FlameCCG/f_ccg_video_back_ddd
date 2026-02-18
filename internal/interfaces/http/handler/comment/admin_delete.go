package comment

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// AdminDeleteCommentReq is an exported API type.
type AdminDeleteCommentReq struct {
	CommentIDs []uint `json:"commentIds" binding:"required,min=1,dive,gt=0"`
}

// AdminDeleteView 管理员批量删除评论.
func (h *Handler) AdminDeleteView(c *gin.Context) {
	req := middlewares.GetBound[AdminDeleteCommentReq](c)
	ctx := c.Request.Context()

	deleted, err := h.svc.AdminDelete(ctx, req.CommentIDs)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithData(map[string]int64{"deleted": deleted}, c)
}
