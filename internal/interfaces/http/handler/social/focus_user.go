package social

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// FocusUserReq 关注用户请求.
type FocusUserReq struct {
	FocusUserID uint `json:"focusUserId" binding:"required" label:"关注用户ID"`
}

// FocusUserView 关注用户.
func (h *Handler) FocusUserView(c *gin.Context) {
	req := middlewares.GetBound[FocusUserReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	if err := h.focus.Focus(c.Request.Context(), uid, req.FocusUserID); err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)
		return
	}

	res.OkWithMsg("关注成功", c)
}
