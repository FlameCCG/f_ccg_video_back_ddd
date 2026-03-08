package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// QQLoginReq 请求/响应结构体.
type QQLoginReq struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state" binding:"required"`
}

// QQLoginView 处理对应 HTTP 请求.
func (h *Handler) QQLoginView(c *gin.Context) {
	if !h.ensureOAuthProviderEnabled(c, app.OAuthProviderQQ) {
		return
	}

	cr := middlewares.GetBound[QQLoginReq](c)

	if err := h.oauth.ConsumeOAuthState(c.Request.Context(), app.OAuthProviderQQ, cr.State); err != nil {
		res.FailWithMsg("OAuth state invalid", c)
		return
	}

	h.completeOAuthLogin(c, oauthLoginParams{
		provider: app.OAuthProviderQQ,
		code:     cr.Code,
	})
}
