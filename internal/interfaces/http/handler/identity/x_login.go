package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// XLoginReq 请求/响应结构体.
type XLoginReq struct {
	Code         string `json:"code" binding:"required"`
	State        string `json:"state" binding:"required"`
	CodeVerifier string `json:"codeVerifier" binding:"required"`
}

// XLoginView 处理对应 HTTP 请求.
func (h *Handler) XLoginView(c *gin.Context) {
	if !h.ensureOAuthProviderEnabled(c, app.OAuthProviderX) {
		return
	}

	cr := middlewares.GetBound[XLoginReq](c)

	if err := h.oauth.ConsumeOAuthState(c.Request.Context(), app.OAuthProviderX, cr.State); err != nil {
		res.FailWithMsg("x登录状态无效", c)

		return
	}

	h.completeOAuthLogin(c, oauthLoginParams{
		provider:     app.OAuthProviderX,
		code:         cr.Code,
		codeVerifier: cr.CodeVerifier,
	})
}
