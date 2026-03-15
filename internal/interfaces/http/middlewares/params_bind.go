package middlewares

import (
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// JsonBindParams 以 JSON 绑定并校验 T，成功后将值保存到 request 上下文键，失败写参数错误并 Abort。
func JsonBindParams[T any](c *gin.Context) {
	LimitRequestBody(c, DefaultJSONBodyLimit)

	var cr T
	if err := c.ShouldBindJSON(&cr); err != nil {
		loggerFromContext(c).Error("JsonBindParams error", zap.Error(err))
		res.FailWithError(err, c)
		c.Abort()

		return
	}

	c.Set("request", cr)
	c.Next()
}

// BindParams 由 Gin 根据请求方式与 Content-Type 选择绑定器，成功后保存 T 供处理器读取。
func BindParams[T any](c *gin.Context) {
	LimitRequestBody(c, 52<<20)
	var cr T
	if err := c.ShouldBind(&cr); err != nil {
		loggerFromContext(c).Error("BindParams error", zap.Error(err))
		res.FailWithError(err, c)
		c.Abort()

		return
	}

	c.Set("request", cr)
	c.Next()
}

// BindQueryParams 只绑定 URL query 到 T，避免将请求体字段误当作该查询入口的参数来源。
func BindQueryParams[T any](c *gin.Context) {
	var cr T
	if err := c.ShouldBindQuery(&cr); err != nil {
		loggerFromContext(c).Error("BindQueryParams error", zap.Error(err))
		res.FailWithError(err, c)
		c.Abort()

		return
	}

	c.Set("request", cr)
	c.Next()
}

// GetBound 取得前置绑定中间件保存的 T；键缺失或类型不同返回 T 的零值。
// 路由与处理器须使用同一个类型，零值回退不是已完成校验的证明。
func GetBound[T any](c *gin.Context) T {
	val, ok := c.Get("request")
	if !ok {
		var zero T
		return zero
	}

	req, ok := val.(T)
	if !ok {
		var zero T
		return zero
	}

	return req
}
