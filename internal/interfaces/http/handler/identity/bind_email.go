package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// BindEmailReq 请求/响应结构体.
type BindEmailReq struct {
	EmailID   string `json:"emailID" binding:"required" label:"邮箱验证码ID"`
	EmailCode string `json:"emailCode" binding:"required" label:"邮箱验证码"`
	Email     string `json:"email" binding:"required" label:"邮箱"`
}

// BindEmailView 绑定邮箱（第三方登录用户）.
func (h *Handler) BindEmailView(c *gin.Context) {
	req := middlewares.GetBound[BindEmailReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	alreadyBound, err := h.accounts.BindEmail(c.Request.Context(), app.BindEmailInput{
		UserID: uid,
		Email:  req.Email,
	})
	if err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)

		return
	}

	if alreadyBound {
		res.OkWithMsg("邮箱已绑定", c)

		return
	}

	res.OkWithMsg("绑定成功", c)
}
