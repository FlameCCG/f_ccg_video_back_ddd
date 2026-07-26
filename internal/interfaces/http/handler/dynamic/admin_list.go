package dynamic

import (
	app "f_ccg_video_back_ddd/internal/application/dynamic"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// AdminDynamicListReq 管理端动态列表请求.
type AdminDynamicListReq struct {
	UserID   uint   `form:"userId" binding:"omitempty,gt=0"`
	Username string `form:"username"`
	Keyword  string `form:"keyword"`
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"pageSize" binding:"omitempty,min=1,max=100"`
}

// AdminListView 管理员动态列表.
func (h *Handler) AdminListView(c *gin.Context) {
	req := middlewares.GetBound[AdminDynamicListReq](c)
	ctx := c.Request.Context()

	list, total, err := h.admin.List(ctx, app.AdminDynamicListParams{
		UserID:   req.UserID,
		Username: req.Username,
		Keyword:  req.Keyword,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithList(list, total, c)
}
