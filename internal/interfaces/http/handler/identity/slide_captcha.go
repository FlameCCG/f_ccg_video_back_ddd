package identity

import (
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"

	"go.uber.org/zap"
)

// SlideCaptchaResponse 请求/响应结构体.
type SlideCaptchaResponse struct {
	MasterImage string `json:"masterImage"`
	TileImage   string `json:"tileImage"`
	Token       string `json:"token"`
	ThumbY      int    `json:"thumbY"`
}

// SlideCaptchaView 处理对应 HTTP 请求.
func (h *CaptchaHandler) SlideCaptchaView(c *gin.Context) {
	scene := c.DefaultQuery("scene", "register")
	if scene != "login" && scene != "register" {
		res.FailWithMsg("invalid captcha scene", c)
		return
	}
	data, err := h.captcha.GenerateSlide(c.Request.Context(), scene)
	if err != nil {
		h.log().Error("generate slide captcha failed", zap.Error(err))
		res.FailWithMsg("生成验证码失败", c)

		return
	}

	res.OkWithData(SlideCaptchaResponse{
		MasterImage: data.MasterImage,
		TileImage:   data.TileImage,
		Token:       data.Token,
		ThumbY:      data.ThumbY,
	}, c)
}
