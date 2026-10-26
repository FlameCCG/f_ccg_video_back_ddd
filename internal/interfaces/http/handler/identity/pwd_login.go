package identity

import (
	"context"
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	jwts "f_ccg_video_back_ddd/pkg/jwt"
	"f_ccg_video_back_ddd/pkg/res"
	"f_ccg_video_back_ddd/pkg/utils/ipUtils"

	"github.com/gin-gonic/gin"
)

// PwdLoginReq 请求/响应结构体.
type PwdLoginReq struct {
	UserName string `json:"username" binding:"required" label:"用户名或邮箱"`
	Password string `json:"password" binding:"required" label:"密码"`
}

// PwdLoginView 处理对应 HTTP 请求.
func (h *Handler) PwdLoginView(c *gin.Context) {
	passwordLogin(c, middlewares.GetBound[PwdLoginReq](c), h.resolver, h.logins.PwdLogin)
}

func passwordLogin(c *gin.Context, cr PwdLoginReq, resolver ipUtils.RegionResolver, authenticate func(context.Context, app.PwdLoginInput) (*jwts.JwtToken, error)) {
	ip, addr := ipUtils.GetClientIPAndAddr(c, resolver)

	token, err := authenticate(c.Request.Context(), app.PwdLoginInput{Identifier: cr.UserName, Password: cr.Password, IP: ip, Addr: addr, UserAgent: c.GetHeader("User-Agent")})
	if err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)
		return
	}

	res.OkWithData(token, c)
}
