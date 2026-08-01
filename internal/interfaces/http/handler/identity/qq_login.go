package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"

	"github.com/gin-gonic/gin"
)

// QQLoginReq 请求/响应结构体.
type QQLoginReq struct {
	Code string `json:"code"`
}

// QQLoginView 处理对应 HTTP 请求.
func (h *Handler) QQLoginView(c *gin.Context) {
	if !h.ensureOAuthProviderEnabled(c, app.OAuthProviderQQ) {
		return
	}

	cr := middlewares.GetBound[QQLoginReq](c)

	// QQ 登录不校验 state（与原实现一致）。
	h.completeOAuthLogin(c, oauthLoginParams{
		provider: app.OAuthProviderQQ,
		code:     cr.Code,
	})
}
