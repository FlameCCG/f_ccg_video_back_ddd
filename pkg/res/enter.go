// Package res 统一 Gin HTTP JSON 响应与参数错误输出。
//
// Response 的业务 code、msg、data 与 HTTP 状态码是不同字段，调用辅助函数前应核对协议约定。
// 分页包装、成功和失败响应在这里统一，校验错误通过当前请求的中文翻译器转换。
package res

import (
	"f_ccg_video_back_ddd/pkg/validate"

	"github.com/gin-gonic/gin"
)

// Response 是统一 JSON 响应信封，业务结果用 Code 表达，Data 承载载荷，Msg 面向调用方展示。
type Response struct {
	Code Code   `json:"code"`
	Data any    `json:"data"`
	Msg  string `json:"msg"`
}

// Code 表示响应体中的业务状态码，不是 net/http 的状态码。
type Code int

const (
	// SuccessCode 表示业务处理成功，取值 0 是现有客户端契约。
	SuccessCode = 0
	// FailServiceCode 表示业务处理失败；当前响应辅助函数仍使用 HTTP 200。
	FailServiceCode = 1
)

// Json 以 HTTP 200 写出当前响应信封，业务成功与否由响应体 Code 区分。
func (r Response) Json(c *gin.Context) {
	c.JSON(200, r)
}

// OkWithData 返回成功载荷，可选文案只取第一个参数，未提供时使用 ok。
func OkWithData(data any, c *gin.Context, msg ...string) {
	if len(msg) > 0 {
		Response{SuccessCode, data, msg[0]}.Json(c)
		return
	}

	Response{SuccessCode, data, "ok"}.Json(c)
}

// OkWithMsg 返回成功文案和空对象载荷，保持客户端对 data 形状的约定。
func OkWithMsg(msg string, c *gin.Context) {
	Response{SuccessCode, map[string]any{}, msg}.Json(c)
}

// OkWithList 将分页列表和总条数包装到 data.list/data.total，不自行执行分页查询。
func OkWithList(list any, total int64, c *gin.Context) {
	Response{SuccessCode, map[string]any{
		"list":  list,
		"total": total,
	}, "成功"}.Json(c)
}

func fail(msg string, c *gin.Context) {
	Response{FailServiceCode, map[string]any{}, msg}.Json(c)
}

// FailWithError 按当前请求的校验翻译器生成可读参数错误；非校验错误退回通用文案。
func FailWithError(err error, c *gin.Context) {
	msg := validate.ErrorFromContext(c, err)
	fail(msg, c)
}

// FailWithMsg 直接返回调用方选择的业务失败文案，载荷为空对象。
func FailWithMsg(msg string, c *gin.Context) {
	fail(msg, c)
}
