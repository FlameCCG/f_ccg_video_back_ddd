package identity

import (
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// CaptchaTypeView 返回当前场景选定的验证码类型。
func (h *CaptchaHandler) CaptchaTypeView(c *gin.Context) {
	c.Header("Cache-Control", "no-store")

	scene := c.Query("scene")
	if scene != "login" && scene != "register" {
		res.FailWithMsg("invalid captcha scene", c)
		return
	}

	options := h.captcha.InteractiveCaptchaOptions(scene)
	res.OkWithData(gin.H{"type": options.SelectType()}, c)
}
