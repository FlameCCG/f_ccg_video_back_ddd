package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// CreateTagReq 请求/响应结构体.
type CreateTagReq struct {
	Name string `json:"name" binding:"required,max=64" label:"标签名称"`
}

// CreateTag 用户创建标签.
func (h *Handler) CreateTag(c *gin.Context) {
	req := middlewares.GetBound[CreateTagReq](c)
	if req.Name == "" {
		res.FailWithMsg("标签名称不能为空", c)

		return
	}

	tag, err := h.activity.CreateTag(c.Request.Context(), req.Name)
	if err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)

		return
	}

	res.OkWithData(tag, c)
}
