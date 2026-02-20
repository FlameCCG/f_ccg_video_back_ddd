package dynamic

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// DeleteDynamicReq 删除动态请求.
type DeleteDynamicReq struct {
	DynamicID uint `json:"dynamicId" binding:"required,gt=0" label:"动态ID"`
}

// DeleteView 删除自己的动态.
func (h *Handler) DeleteView(c *gin.Context) {
	req := middlewares.GetBound[DeleteDynamicReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	if err := h.dynamics.Delete(c.Request.Context(), uid, req.DynamicID); err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithMsg("删除成功", c)
}
