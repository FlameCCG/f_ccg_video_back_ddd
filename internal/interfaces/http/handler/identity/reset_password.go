package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// ResetPasswordReq 请求/响应结构体.
type ResetPasswordReq struct {
	EmailID     string `json:"emailID" binding:"required" label:"邮箱验证码ID"`
	EmailCode   string `json:"emailCode" binding:"required" label:"邮箱验证码"`
	Email       string `json:"email" binding:"required" label:"邮箱"`
	NewPassword string `json:"newPassword" binding:"required" label:"新密码"`
}

// ResetPasswordView 忘记密码（邮箱验证码）.
func (h *Handler) ResetPasswordView(c *gin.Context) {
	req := middlewares.GetBound[ResetPasswordReq](c)

	if err := h.accounts.ResetPassword(c.Request.Context(), app.ResetPasswordInput{
		Email:       req.Email,
		NewPassword: req.NewPassword,
	}); err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithMsg("重置成功", c)
}
