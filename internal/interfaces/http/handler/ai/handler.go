// Package ai 把模型选项、聊天流、图片、视频任务与上游资源的 HTTP 请求翻译为应用用例调用。
//
// handler.go 声明显式依赖，其他文件按入口拆分；参数结构体保留 Gin 绑定与校验标签。
// 从请求取得参数和经过鉴权的身份，构造应用参数后传 c.Request.Context()，最后映射错误和响应。
// gin.Context 止步于接口层，领域规则、事务、缓存与跨上下文调用由 application 编排。
// responses.go 等流接口需要传播取消并按上游事件发送；流开始后错误必须按流协议表达。
// JSON 字段、状态码和响应文案属于对外契约，新增注释不应顺带调整这些行为。
package ai

import (
	aiapp "f_ccg_video_back_ddd/internal/application/ai"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler AI HTTP 处理器。
//
// 原实现是 `AIAPI struct{ provider ai_service.Provider }` + 手写 NewAIAPI；
// 新架构由 wire 注入应用服务。
type Handler struct {
	service *aiapp.Service
	logger  *zap.Logger
}

// NewHandler 构造 AI HTTP 处理器。
func NewHandler(service *aiapp.Service, logger *zap.Logger) *Handler {
	return &Handler{service: service, logger: loggerOrNop(logger)}
}

func (h *Handler) log() *zap.Logger {
	if h == nil {
		return zap.NewNop()
	}

	return loggerOrNop(h.logger)
}

func loggerOrNop(logger *zap.Logger) *zap.Logger {
	if logger == nil {
		return zap.NewNop()
	}

	return logger
}

// bound 读取中间件已绑定的请求参数。
func bound[T any](c *gin.Context) T {
	return middlewares.GetBound[T](c)
}
