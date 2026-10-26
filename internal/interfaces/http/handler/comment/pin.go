package comment

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// PinReq 表达把指定评论设为或取消置顶，Pinned=false 是明确的取消操作。
type PinReq struct {
	CommentID uint `json:"commentId" binding:"required,gt=0" label:"评论ID"`
	Pinned    bool `json:"pinned" label:"是否置顶"`
}

// PinView 作者置顶/取消置顶评论.
func (h *Handler) PinView(c *gin.Context) {
	req := middlewares.GetBound[PinReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.svc.Pin(c.Request.Context(), uid, req.CommentID, req.Pinned); err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithMsg("操作成功", c)
}
