package identity

import (
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// ClickCaptchaResponse 请求/响应结构体.
type ClickCaptchaResponse struct {
	MasterImage string `json:"masterImage"`
	ThumbImage  string `json:"thumbImage"`
	Token       string `json:"token"`
}

// ClickCaptchaView 处理对应 HTTP 请求.
func (h *CaptchaHandler) ClickCaptchaView(c *gin.Context) {
	scene := c.DefaultQuery("scene", "login")
	if scene != "login" && scene != "register" {
		res.FailWithMsg("invalid captcha scene", c)
		return
	}

	data, err := h.captcha.GenerateClick(c.Request.Context(), scene)
	if err != nil {
		res.RecordFailure(apperr.Wrap("handler/identity/click_captcha", nil, err), c)
		res.FailWithCause(err, "生成验证码失败", c)

		return
	}

	res.OkWithData(ClickCaptchaResponse{
		MasterImage: data.MasterImage,
		ThumbImage:  data.ThumbImage,
		Token:       data.Token,
	}, c)
}
