// Package ai 是 AI 上下文的 HTTP 处理器（原 api/ai_api）。
package ai

import (
	aiapp "f_ccg_video_back_ddd/internal/application/ai"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"

	"github.com/gin-gonic/gin"
)

// Handler AI HTTP 处理器。
//
// 原实现是 `AIAPI struct{ provider ai_service.Provider }` + 手写 NewAIAPI；
// 新架构由 wire 注入应用服务。
type Handler struct {
	service *aiapp.Service
}

// NewHandler 构造 AI HTTP 处理器。
func NewHandler(service *aiapp.Service) *Handler {
	return &Handler{service: service}
}

// bound 读取中间件已绑定的请求参数。
func bound[T any](c *gin.Context) T {
	return middlewares.GetBound[T](c)
}
