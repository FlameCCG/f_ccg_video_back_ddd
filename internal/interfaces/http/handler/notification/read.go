package notification

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// MarkReadReq is an exported API type.
type MarkReadReq struct {
	IDs []uint `json:"ids" binding:"required,min=1,dive,gt=0" label:"通知ID列表"`
}

// MarkRead handles the corresponding HTTP request.
func (h *Handler) MarkRead(c *gin.Context) {
	req := middlewares.GetBound[MarkReadReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.svc.MarkRead(c.Request.Context(), int(uid), req.IDs); err != nil {
		res.FailWithMsg("更新失败", c)

		return
	}

	res.OkWithMsg("更新成功", c)
}
