package identity

import (
	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// AdminUserBanReq 请求/响应结构体.
type AdminUserBanReq struct {
	UserID uint            `json:"userId" binding:"required,gt=0"`
	Days   int             `json:"days" binding:"omitempty,min=0"`
	Reason string          `json:"reason"`
	Status enum.UserStatus `json:"status" binding:"omitempty"`
}

// AdminBanView 管理员封禁/解封用户.
func (h *AdminHandler) AdminBanView(c *gin.Context) {
	req := middlewares.GetBound[AdminUserBanReq](c)

	if !ensureUserStatusValid(c, req.Status) {
		return
	}

	// status 为 0（未传）时交给领域层按 days 推导目标状态。
	operatorID, _ := middlewares.GetUID(c)

	record, err := h.bans.Apply(c.Request.Context(), domain.BanAction{
		UserID:     req.UserID,
		Status:     req.Status,
		Days:       req.Days,
		Reason:     req.Reason,
		OperatorID: operatorID,
	})
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithData(record, c)
}
