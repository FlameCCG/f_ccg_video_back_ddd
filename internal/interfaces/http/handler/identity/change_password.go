package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// ChangePasswordReq 请求/响应结构体.
type ChangePasswordReq struct {
	OldPassword string `json:"oldPassword" label:"旧密码"`
	NewPassword string `json:"newPassword" binding:"required" label:"新密码"`
}

// ChangePasswordView 修改密码（需登录）.
func (h *Handler) ChangePasswordView(c *gin.Context) {
	req := middlewares.GetBound[ChangePasswordReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.accounts.ChangePassword(c.Request.Context(), app.ChangePasswordInput{
		UserID:      uid,
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	}); err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)

		return
	}

	res.OkWithMsg("修改成功", c)
}
