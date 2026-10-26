package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"

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

	h.completeOAuthCallback(c, cr.State, "x登录状态无效", oauthLoginParams{
		provider:     app.OAuthProviderX,
		code:         cr.Code,
		codeVerifier: cr.CodeVerifier,
	})
}
