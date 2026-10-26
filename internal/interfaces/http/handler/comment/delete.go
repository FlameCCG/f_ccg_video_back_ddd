package comment

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// DeleteCommentReq 指定一条待删除评论；是否有权删除还需用例校验评论者/作品作者身份。
type DeleteCommentReq struct {
	CommentID uint `json:"commentId" binding:"required" label:"评论ID"`
}

// DeleteView 将当前用户与评论 ID 交给删除用例，不直接执行 SQL 级联清理。
func (h *Handler) DeleteView(c *gin.Context) {
	req := middlewares.GetBound[DeleteCommentReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.svc.Delete(c.Request.Context(), uid, req.CommentID); err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)

		return
	}

	res.OkWithMsg("删除成功", c)
}
