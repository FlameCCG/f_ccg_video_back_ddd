package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"

	"go.uber.org/zap"
)

// GoogleLoginReq 请求/响应结构体.
type GoogleLoginReq struct {
	Code  string `json:"code" binding:"required" label:"授权码"`
	State string `json:"state" binding:"required" label:"state"`
}

// GoogleLoginView 处理对应 HTTP 请求.
func (h *Handler) GoogleLoginView(c *gin.Context) {
	if !h.ensureOAuthProviderEnabled(c, app.OAuthProviderGoogle) {
		return
	}

	cr := middlewares.GetBound[GoogleLoginReq](c)

	if err := h.oauth.ConsumeOAuthState(c.Request.Context(), app.OAuthProviderGoogle, cr.State); err != nil {
		h.log().Warn("Google OAuth State Invalid", zap.Error(err))
		res.FailWithMsg("google登录状态无效，请重试", c)

		return
	}

	h.completeOAuthLogin(c, oauthLoginParams{
		provider: app.OAuthProviderGoogle,
		code:     cr.Code,
	})
}
