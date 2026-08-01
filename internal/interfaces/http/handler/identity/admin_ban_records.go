package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// AdminBanRecordListReq 请求/响应结构体.
type AdminBanRecordListReq struct {
	UserID   uint            `form:"userId" binding:"omitempty,gt=0"`
	Username string          `form:"username"`
	Status   enum.UserStatus `form:"status" binding:"omitempty"`
	Page     int             `form:"page" binding:"omitempty,min=1"`
	PageSize int             `form:"pageSize" binding:"omitempty,min=1,max=100"`
}

// AdminBanRecordListView 管理员封禁记录列表.
func (h *AdminHandler) AdminBanRecordListView(c *gin.Context) {
	req := middlewares.GetBound[AdminBanRecordListReq](c)

	if !ensureUserStatusValid(c, req.Status) {
		return
	}

	list, total, err := h.records.AdminList(c.Request.Context(), app.AdminBanRecordListParams{
		UserID:   req.UserID,
		Username: req.Username,
		Status:   req.Status,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithList(list, total, c)
}
