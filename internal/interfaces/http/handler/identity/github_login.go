package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// GitHubLoginReq 请求/响应结构体.
type GitHubLoginReq struct {
	Code         string `json:"code" binding:"required"`
	State        string `json:"state" binding:"required"`
	CodeVerifier string `json:"codeVerifier"`
}

// GitHubLoginView 处理对应 HTTP 请求.
func (h *Handler) GitHubLoginView(c *gin.Context) {
	if !h.ensureOAuthProviderEnabled(c, app.OAuthProviderGitHub) {
		return
	}

	cr := middlewares.GetBound[GitHubLoginReq](c)

	if err := h.oauth.ConsumeOAuthState(c.Request.Context(), app.OAuthProviderGitHub, cr.State); err != nil {
		res.FailWithMsg("github登录状态无效", c)

		return
	}

	h.completeOAuthLogin(c, oauthLoginParams{
		provider:     app.OAuthProviderGitHub,
		code:         cr.Code,
		codeVerifier: cr.CodeVerifier,
	})
}
