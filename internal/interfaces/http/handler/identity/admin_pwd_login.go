package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"

	"github.com/gin-gonic/gin"
)

// AdminPwdLoginReq 请求/响应结构体.
type AdminPwdLoginReq = PwdLoginReq

// AdminPwdLoginView 管理员用户名密码登录.
func (h *AdminHandler) AdminPwdLoginView(c *gin.Context) {
	passwordLogin(c, middlewares.GetBound[AdminPwdLoginReq](c), h.resolver, h.logins.AdminPwdLogin)
}
