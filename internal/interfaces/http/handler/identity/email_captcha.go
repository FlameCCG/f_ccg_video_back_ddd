package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// EmailCaptchaRequest 请求/响应结构体.
type EmailCaptchaRequest struct {
	Type  int    `form:"type" json:"type" binding:"required,oneof=1 2 3"` // 1注册 2重置密码 3绑定邮箱
	Email string `form:"email" json:"email" binding:"required"`
}

// EmailCaptchaResponse 请求/响应结构体.
type EmailCaptchaResponse struct {
	EmailID string `json:"emailID"`
}

// EmailCaptchaView 发送邮箱验证码（限流 + 邮箱可达性校验 + 异步投递）.
func (h *CaptchaHandler) EmailCaptchaView(c *gin.Context) {
	cr := middlewares.GetBound[EmailCaptchaRequest](c)

	result, err := h.emails.Send(c.Request.Context(), app.EmailCaptchaSendInput{
		Type:     cr.Type,
		Email:    cr.Email,
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithData(EmailCaptchaResponse{
		EmailID: result.EmailID,
	}, c, "验证码发送中，请稍候查收")
}
