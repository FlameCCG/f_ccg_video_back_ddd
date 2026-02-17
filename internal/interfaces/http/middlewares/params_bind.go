package middlewares

import (
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// JsonBindParams 函数.
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

// BindParams 函数.
func BindParams[T any](c *gin.Context) {
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

// BindQueryParams 函数.
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

// GetBound 函数.
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
