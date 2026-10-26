package identity

import (
	"errors"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/pkg/res"
	"f_ccg_video_back_ddd/pkg/utils/ipUtils"

	app "f_ccg_video_back_ddd/internal/application/identity"

	"github.com/gin-gonic/gin"
)

// oauthLoginParams 第三方登录 handler 的公共参数。
type oauthLoginParams struct {
	// provider 提供商标识（同时是 register_source 与错误文案前缀）。
	provider string
	// code 授权码。
	code string
	// codeVerifier PKCE 校验串（github 可选、x 必填，其余为空）。
	codeVerifier string
}

// completeOAuthLogin 执行第三方登录并写响应。
//
// 等价原 api/user_api/oauth_login_helper.go 的 completeOAuthLogin：
// 换取用户信息与查找/创建账号失败统一回「{provider}登录失败」，
// 封禁与签发失败按应用层文案透出。
func (h *Handler) completeOAuthLogin(c *gin.Context, params oauthLoginParams) {
	ip, addr := ipUtils.GetClientIPAndAddr(c, h.resolver)

	jwtToken, err := h.oauth.Login(c.Request.Context(), app.OAuthLoginInput{
		Provider:     params.provider,
		Code:         params.code,
		CodeVerifier: params.codeVerifier,
		IP:           ip,
		Addr:         addr,
		UserAgent:    c.GetHeader("User-Agent"),
	})
	if err != nil {
		if errors.Is(err, app.ErrOAuthProviderFailed) || errors.Is(err, app.ErrOAuthProviderUnsupported) {
			res.FailWithCause(err, params.provider+"登录失败", c)

			return
		}

		res.FailWithCause(err, clienterror.Message(err), c)

		return
	}

	res.OkWithData(jwtToken, c)
}

// ensureOAuthProviderEnabled 校验站点是否开启该第三方登录；未开启时已写响应并返回 false。
func (h *Handler) ensureOAuthProviderEnabled(c *gin.Context, provider string) bool {
	if h.oauth.ProviderEnabled(provider) {
		return true
	}

	res.FailWithMsg(provider+"登录已关闭", c)

	return false
}

// completeOAuthCallback consumes the provider-bound state before authenticating the callback.
func (h *Handler) completeOAuthCallback(c *gin.Context, state, invalidStateMessage string, params oauthLoginParams) {
	if err := h.oauth.ConsumeOAuthState(c.Request.Context(), params.provider, state); err != nil {
		res.FailWithCause(err, invalidStateMessage, c)
		return
	}

	h.completeOAuthLogin(c, params)
}
