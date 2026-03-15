package dynamic

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// CreateDynamicReq 发布动态请求.
type CreateDynamicReq struct {
	Content  string `json:"content" binding:"required,max=2000" label:"内容"`
	ImageURL string `json:"imageUrl" binding:"omitempty,max=500" label:"图片URL"`
}

// CreateView 发布动态.
func (h *Handler) CreateView(c *gin.Context) {
	req := middlewares.GetBound[CreateDynamicReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	item, err := h.dynamics.Create(c.Request.Context(), uid, req.Content, req.ImageURL)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)
		return
	}

	res.OkWithData(item, c)
}
