package notification

import (
	"errors"

	domain "f_ccg_video_back_ddd/internal/domain/notification"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// DeleteNotificationReq is an exported API type.
type DeleteNotificationReq struct {
	IDs []uint `json:"ids" binding:"required,min=1,dive,gt=0" label:"通知ID列表"`
}

// Delete handles the corresponding HTTP request.
func (h *Handler) Delete(c *gin.Context) {
	req := middlewares.GetBound[DeleteNotificationReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.svc.Delete(c.Request.Context(), int(uid), req.IDs); err != nil {
		if errors.Is(err, domain.ErrSystemNotificationNotDeletable) {
			res.FailWithMsg(err.Error(), c)

			return
		}

		res.FailWithMsg("删除失败", c)

		return
	}

	res.OkWithMsg("删除成功", c)
}

// AdminDelete 仅管理员可用.
func (h *Handler) AdminDelete(c *gin.Context) {
	req := middlewares.GetBound[DeleteNotificationReq](c)

	if err := h.svc.AdminDelete(c.Request.Context(), req.IDs); err != nil {
		res.FailWithMsg("删除失败", c)

		return
	}

	res.OkWithMsg("删除成功", c)
}
