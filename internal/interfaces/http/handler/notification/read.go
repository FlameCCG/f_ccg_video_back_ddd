package notification

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// MarkReadReq 指定需要标记已读的通知 ID，不直接修改通知正文或其他用户的阅读状态。
type MarkReadReq struct {
	IDs []uint `json:"ids" binding:"required,min=1,dive,gt=0" label:"通知ID列表"`
}

// MarkRead 以当前登录身份标记指定通知已读，并由应用层维护角标失效。
func (h *Handler) MarkRead(c *gin.Context) {
	req := middlewares.GetBound[MarkReadReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.svc.MarkRead(c.Request.Context(), int(uid), req.IDs); err != nil {
		res.FailWithCause(err, "更新失败", c)

		return
	}

	res.OkWithMsg("更新成功", c)
}
