package identity

import (
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"

	"go.uber.org/zap"
)

// ClickCaptchaResponse 请求/响应结构体.
type ClickCaptchaResponse struct {
	MasterImage string `json:"masterImage"`
	ThumbImage  string `json:"thumbImage"`
	Token       string `json:"token"`
}

// ClickCaptchaView 处理对应 HTTP 请求.
func (h *CaptchaHandler) ClickCaptchaView(c *gin.Context) {
	data, err := h.captcha.GenerateClick(c.Request.Context())
	if err != nil {
		h.log().Error("generate click captcha failed", zap.Error(err))
		res.FailWithMsg("生成验证码失败", c)

		return
	}

	res.OkWithData(ClickCaptchaResponse{
		MasterImage: data.MasterImage,
		ThumbImage:  data.ThumbImage,
		Token:       data.Token,
	}, c)
}
