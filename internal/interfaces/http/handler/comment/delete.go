package comment

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// DeleteCommentReq is an exported API type.
type DeleteCommentReq struct {
	CommentID uint `json:"commentId" binding:"required" label:"评论ID"`
}

// DeleteView handles the corresponding HTTP request.
func (h *Handler) DeleteView(c *gin.Context) {
	req := middlewares.GetBound[DeleteCommentReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.svc.Delete(c.Request.Context(), uid, req.CommentID); err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithMsg("删除成功", c)
}
