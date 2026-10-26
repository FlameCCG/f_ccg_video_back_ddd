package comment

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// AdminDeleteCommentReq 携带管理端批量删除的评论 ID；是否允许管理操作由路由鉴权链保证。
type AdminDeleteCommentReq struct {
	CommentIDs []uint `json:"commentIds" binding:"required,min=1,dive,gt=0"`
}

// AdminDeleteView 管理员批量删除评论.
func (h *Handler) AdminDeleteView(c *gin.Context) {
	req := middlewares.GetBound[AdminDeleteCommentReq](c)
	ctx := c.Request.Context()

	deleted, err := h.svc.AdminDelete(ctx, req.CommentIDs)
	if err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)

		return
	}

	res.OkWithData(map[string]int64{"deleted": deleted}, c)
}
