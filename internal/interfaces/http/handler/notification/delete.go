package notification

import (
	"errors"

	domain "f_ccg_video_back_ddd/internal/domain/notification"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// DeleteNotificationReq 指定当前用户要删除的通知集合，系统/全员通知的删除限制由应用用例执行。
type DeleteNotificationReq struct {
	IDs []uint `json:"ids" binding:"required,min=1,dive,gt=0" label:"通知ID列表"`
}

// Delete 把登录用户和通知 ID 集合交给删除用例，避免请求体自报接收者身份。
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
