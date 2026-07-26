// Package res provides package functionality.
package res

import (
	"f_ccg_video_back_ddd/pkg/validate"

	"github.com/gin-gonic/gin"
)

// Response 类型.
type Response struct {
	Code Code   `json:"code"`
	Data any    `json:"data"`
	Msg  string `json:"msg"`
}

// Code 类型.
type Code int

const (
	// SuccessCode 常量.
	SuccessCode = 0
	// FailServiceCode 常量.
	FailServiceCode = 1
)

var empty = map[string]any{}

// Json 方法.
func (r Response) Json(c *gin.Context) {
	c.JSON(200, r)
}

// OkWithData 导出符号.
func OkWithData(data any, c *gin.Context, msg ...string) {
	if len(msg) > 0 {
		Response{SuccessCode, data, msg[0]}.Json(c)
		return
	}

	Response{SuccessCode, data, "ok"}.Json(c)
}

// OkWithMsg 导出符号.
func OkWithMsg(msg string, c *gin.Context) {
	Response{SuccessCode, empty, msg}.Json(c)
}

// OkWithList 导出符号.
func OkWithList(list any, total int64, c *gin.Context) {
	Response{SuccessCode, map[string]any{
		"list":  list,
		"total": total,
	}, "成功"}.Json(c)
}

func fail(msg string, c *gin.Context) {
	Response{FailServiceCode, empty, msg}.Json(c)
}

// FailWithError 导出符号.
func FailWithError(err error, c *gin.Context) {
	msg := validate.ValidateError(err)
	fail(msg, c)
}

// FailWithMsg 导出符号.
func FailWithMsg(msg string, c *gin.Context) {
	fail(msg, c)
}
