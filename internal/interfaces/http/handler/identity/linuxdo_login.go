package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

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

	if err := h.oauth.ConsumeOAuthState(c.Request.Context(), app.OAuthProviderLinuxDo, cr.State); err != nil {
		res.FailWithMsg("linuxdo登录状态无效", c)

		return
	}

	h.completeOAuthLogin(c, oauthLoginParams{
		provider: app.OAuthProviderLinuxDo,
		code:     cr.Code,
	})
}
