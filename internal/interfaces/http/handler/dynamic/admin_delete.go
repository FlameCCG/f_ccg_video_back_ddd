package dynamic

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// AdminDeleteDynamicReq 管理端批量删除动态请求.
type AdminDeleteDynamicReq struct {
	DynamicIDs []uint `json:"dynamicIds" binding:"required,min=1,dive,gt=0"`
}

// AdminDeleteView 管理员批量删除动态.
func (h *Handler) AdminDeleteView(c *gin.Context) {
	req := middlewares.GetBound[AdminDeleteDynamicReq](c)
	ctx := c.Request.Context()

	deleted, err := h.admin.Delete(ctx, req.DynamicIDs)
	if err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)
		return
	}

	res.OkWithData(map[string]int64{"deleted": deleted}, c)
}
