package dynamic

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// PinDynamicReq 置顶请求：dynamicId 与 videoId 二选一.
type PinDynamicReq struct {
	DynamicID uint `json:"dynamicId" binding:"omitempty,gt=0" label:"动态ID"`
	VideoID   uint `json:"videoId" binding:"omitempty,gt=0" label:"视频ID"`
	Pinned    bool `json:"pinned" label:"是否置顶"`
}

// PinView 置顶/取消置顶动态或视频（二者互斥）.
func (h *Handler) PinView(c *gin.Context) {
	req := middlewares.GetBound[PinDynamicReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	if (req.DynamicID > 0 && req.VideoID > 0) || (req.DynamicID == 0 && req.VideoID == 0) {
		res.FailWithMsg("请传入 dynamicId 或 videoId", c)
		return
	}

	if req.DynamicID > 0 {
		if err := h.dynamics.Pin(c.Request.Context(), uid, req.DynamicID, req.Pinned); err != nil {
			res.FailWithMsg(clienterror.Message(err), c)
			return
		}
	} else {
		if err := h.dynamics.PinVideo(c.Request.Context(), uid, req.VideoID, req.Pinned); err != nil {
			res.FailWithMsg(clienterror.Message(err), c)
			return
		}
	}

	res.OkWithMsg("操作成功", c)
}
