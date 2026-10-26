package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"

	"github.com/gin-gonic/gin"
)

// LinuxDoLoginReq 请求/响应结构体.
type LinuxDoLoginReq struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state" binding:"required"`
}

// LinuxDoLoginView 处理对应 HTTP 请求.
func (h *Handler) LinuxDoLoginView(c *gin.Context) {
	if !h.ensureOAuthProviderEnabled(c, app.OAuthProviderLinuxDo) {
		return
	}

	cr := middlewares.GetBound[LinuxDoLoginReq](c)

	h.completeOAuthCallback(c, cr.State, "linuxdo登录状态无效", oauthLoginParams{
		provider: app.OAuthProviderLinuxDo,
		code:     cr.Code,
	})
}
