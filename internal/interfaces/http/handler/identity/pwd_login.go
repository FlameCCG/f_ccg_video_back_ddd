package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
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
	cr := middlewares.GetBound[PwdLoginReq](c)

	ip, addr := ipUtils.GetClientIPAndAddr(c, h.resolver)

	jwtToken, err := h.logins.PwdLogin(c.Request.Context(), app.PwdLoginInput{
		Identifier: cr.UserName,
		Password:   cr.Password,
		IP:         ip,
		Addr:       addr,
		UserAgent:  c.GetHeader("User-Agent"),
	})
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithData(jwtToken, c)
}
