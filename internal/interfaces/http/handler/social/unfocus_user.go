package social

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// UnfocusUserReq 取消关注用户请求.
type UnfocusUserReq struct {
	FocusUserID uint `json:"focusUserId" binding:"required" label:"取消关注用户ID"`
}

// UnfocusUserView 取消关注用户.
func (h *Handler) UnfocusUserView(c *gin.Context) {
	req := middlewares.GetBound[UnfocusUserReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	if err := h.focus.Unfocus(c.Request.Context(), uid, req.FocusUserID); err != nil {
		res.FailWithMsg(clienterror.Message(err), c)
		return
	}

	res.OkWithMsg("取消关注成功", c)
}
