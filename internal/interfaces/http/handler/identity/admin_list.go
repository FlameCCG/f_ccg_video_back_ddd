package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// AdminUserListReq 请求/响应结构体.
type AdminUserListReq struct {
	Keyword  string          `form:"keyword"`
	Status   enum.UserStatus `form:"status" binding:"omitempty"`
	RoleID   uint            `form:"roleId" binding:"omitempty,gt=0"`
	Page     int             `form:"page" binding:"omitempty,min=1"`
	PageSize int             `form:"pageSize" binding:"omitempty,min=1,max=100"`
}

// AdminUserListView 管理员用户列表.
func (h *AdminHandler) AdminUserListView(c *gin.Context) {
	req := middlewares.GetBound[AdminUserListReq](c)

	if !ensureUserStatusValid(c, req.Status) {
		return
	}

	list, total, err := h.users.List(c.Request.Context(), app.AdminUserListParams{
		Keyword:  req.Keyword,
		Status:   req.Status,
		RoleID:   req.RoleID,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithList(list, total, c)
}

// ensureUserStatusValid 校验可选的用户状态过滤值；非法时已写响应并返回 false。
//
// 与原实现一致：status 为 0 视为「不过滤」，其余只接受 1/2/3。
func ensureUserStatusValid(c *gin.Context, status enum.UserStatus) bool {
	if status == 0 {
		return true
	}

	switch status {
	case enum.UserStatusActive, enum.UserStatusBanned, enum.UserStatusPermanentlyBanned:
		return true
	default:
		res.FailWithMsg("status 非法", c)

		return false
	}
}
