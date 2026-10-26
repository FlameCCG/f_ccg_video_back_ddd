// Package res 统一 Gin HTTP JSON 响应与参数错误输出。
//
// Response 的业务 code、msg、data 与 HTTP 状态码是不同字段，调用辅助函数前应核对协议约定。
// 分页包装、成功和失败响应在这里统一，校验错误通过当前请求的中文翻译器转换。
package res

import (
	"errors"
	"f_ccg_video_back_ddd/pkg/apperr"
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

// JSON 以 HTTP 200 写出当前响应信封，业务成功与否由响应体 Code 区分。
func (r Response) JSON(c *gin.Context) {
	SetSuccess(c, r.Code == SuccessCode)
	c.JSON(200, r)
}

// Json preserves the existing response helper API and registers the business result.
//
//nolint:revive // Existing callers use this spelling; JSON is the preferred method for new code.
func (r Response) Json(c *gin.Context) { r.JSON(c) }

// OkWithData 返回成功载荷，可选文案只取第一个参数，未提供时使用 ok。
func OkWithData(data any, c *gin.Context, msg ...string) {
	if len(msg) > 0 {
		Response{SuccessCode, data, msg[0]}.JSON(c)
		return
	}

	Response{SuccessCode, data, "ok"}.JSON(c)
}

// OkWithMsg 返回成功文案和空对象载荷，保持客户端对 data 形状的约定。
func OkWithMsg(msg string, c *gin.Context) {
	Response{SuccessCode, map[string]any{}, msg}.JSON(c)
}

// OkWithList 将分页列表和总条数包装到 data.list/data.total，不自行执行分页查询。
func OkWithList(list any, total int64, c *gin.Context) {
	Response{SuccessCode, map[string]any{
		"list":  list,
		"total": total,
	}, "成功"}.JSON(c)
}

func fail(msg string, c *gin.Context) {
	Response{FailServiceCode, map[string]any{}, msg}.JSON(c)
}

// FailWithError 按当前请求的校验翻译器生成可读参数错误；非校验错误退回通用文案。
func FailWithError(err error, c *gin.Context) {
	msg := validate.ErrorFromContext(c, err)
	if err == nil {
		RecordFailure(nil, c)
	} else {
		RecordFailure(apperr.Wrap("request.bind", nil, err, apperr.Validation), c)
	}

	fail(msg, c)
}

const (
	successKey = "res.success"
	failureKey = "res.failure"
	stackKey   = "res.stack"
)

// SetSuccess explicitly records stream and non-JSON outcomes without writing a response.
func SetSuccess(c *gin.Context, success bool) { c.Set(successKey, success) }

// Success returns the explicitly registered business result, if any.
func Success(c *gin.Context) (bool, bool) {
	value, exists := c.Get(successKey)
	result, ok := value.(bool)

	return result, exists && ok
}

// RecordFailure retains the original error for the request boundary, including nil misuse.
func RecordFailure(err error, c *gin.Context) {
	if err == nil {
		err = apperr.Wrap("res.nil_cause", nil, errors.New("nil failure cause"), apperr.Internal)
	}
	// Keep the first failure; secondary stream write errors must not hide its cause.
	if _, exists := c.Get(failureKey); !exists {
		c.Set(failureKey, err)
	}

	SetSuccess(c, false)
}

// Failure returns the original registered request failure.
func Failure(c *gin.Context) error {
	value, _ := c.Get(failureKey)
	err, _ := value.(error)

	return err
}

// SetStack registers a panic stack for the request logging boundary.
func SetStack(c *gin.Context, stack string) { c.Set(stackKey, stack) }

// Stack returns the separately registered panic stack.
func Stack(c *gin.Context) string { return c.GetString(stackKey) }

// FailWithCause registers an error and preserves the existing failure envelope.
func FailWithCause(err error, msg string, c *gin.Context) { RecordFailure(err, c); fail(msg, c) }

// FailWithMsg 直接返回调用方选择的业务失败文案，载荷为空对象。
func FailWithMsg(msg string, c *gin.Context) {
	fail(msg, c)
}

// RecordPanic overrides an earlier business refusal so a later panic is never hidden.
func RecordPanic(err error, stack string, c *gin.Context) {
	c.Set(failureKey, err)
	SetStack(c, stack)
	SetSuccess(c, false)
}
