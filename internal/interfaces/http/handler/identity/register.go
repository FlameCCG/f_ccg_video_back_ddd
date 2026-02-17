package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"
	"f_ccg_video_back_ddd/pkg/utils/ipUtils"

	"github.com/gin-gonic/gin"
)

// RegisterReq 请求/响应结构体.
type RegisterReq struct {
	Username string `json:"username" binding:"required" label:"用户名"`
	Password string `json:"password" binding:"required" label:"密码"`
	Email    string `json:"email" binding:"required" label:"邮箱"`
}

// EmailRegisterView 处理对应 HTTP 请求.
func (h *Handler) EmailRegisterView(c *gin.Context) {
	cr := middlewares.GetBound[RegisterReq](c)

	// 使用 GetClientIPAndAddr 解析客户端 IP 与归属地。
	ip, addr := ipUtils.GetIPAndAddr(c, h.resolver)

	if err := h.registration.EmailRegister(c.Request.Context(), app.EmailRegisterInput{
		Username:     cr.Username,
		Password:     cr.Password,
		Email:        cr.Email,
		RegisterIP:   ip,
		RegisterAddr: addr,
	}); err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithMsg("注册成功", c)
}
