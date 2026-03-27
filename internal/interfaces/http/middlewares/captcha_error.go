package middlewares

import (
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// captchaFailure 保留面向用户的具体文案，并让客户端按稳定字段选择原地重试或返回表单。
func captchaFailure(c *gin.Context, kind, reason, message string) {
	res.Response{Code: res.FailServiceCode, Data: map[string]string{
		"captchaType":   kind,
		"captchaReason": reason,
	}, Msg: message}.Json(c)
	c.Abort()
}
