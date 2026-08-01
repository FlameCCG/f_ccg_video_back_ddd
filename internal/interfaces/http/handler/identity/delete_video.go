package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// DeleteVideoReq 请求/响应结构体.
type DeleteVideoReq struct {
	VideoID uint `json:"videoId" binding:"required,gt=0" label:"视频ID"`
}

// DeleteVideoView 用户删除自己的视频（逻辑删除）.
func (h *Handler) DeleteVideoView(c *gin.Context) {
	req := middlewares.GetBound[DeleteVideoReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.activity.DeleteVideo(c.Request.Context(), req.VideoID, uid); err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithMsg("删除成功", c)
}
