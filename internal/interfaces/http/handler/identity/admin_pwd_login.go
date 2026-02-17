package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"
	"f_ccg_video_back_ddd/pkg/utils/ipUtils"

	"github.com/gin-gonic/gin"
)

// AdminPwdLoginReq 请求/响应结构体.
type AdminPwdLoginReq struct {
	UserName string `json:"username" binding:"required" label:"用户名或邮箱"`
	Password string `json:"password" binding:"required" label:"密码"`
}

// AdminPwdLoginView 管理员用户名密码登录.
func (h *AdminHandler) AdminPwdLoginView(c *gin.Context) {
	cr := middlewares.GetBound[AdminPwdLoginReq](c)

	ip, addr := ipUtils.GetClientIPAndAddr(c, h.resolver)

	jwtToken, err := h.logins.AdminPwdLogin(c.Request.Context(), app.PwdLoginInput{
		Identifier: cr.UserName,
		Password:   cr.Password,
		IP:         ip,
		Addr:       addr,
		UserAgent:  c.GetHeader("User-Agent"),
	})
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithData(jwtToken, c)
}
