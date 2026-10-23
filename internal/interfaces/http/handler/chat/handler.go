// Package chat 把私信会话、历史、已读和 WebSocket 升级的 HTTP 请求翻译为应用用例调用。
//
// handler.go 声明显式依赖，其他文件按入口拆分；参数结构体保留 Gin 绑定与校验标签。
// 从请求取得参数和经过鉴权的身份，构造应用参数后传 c.Request.Context()，最后映射错误和响应。
// gin.Context 止步于接口层，领域规则、事务、缓存与跨上下文调用由 application 编排。
// ws.go 只负责把 HTTP 升级请求交给 Hub，长连接的读写和关闭由 interfaces/ws/chat 管理。
// JSON 字段、状态码和响应文案属于对外契约，新增注释不应顺带调整这些行为。
package chat

import (
	chatapp "f_ccg_video_back_ddd/internal/application/chat"
	wschat "f_ccg_video_back_ddd/internal/interfaces/ws/chat"
)

// Handler 私信 HTTP 处理器。
//
// 原实现是无字段的 `ChatAPI struct{}` + 直接 new service；
// 新架构由 wire 注入应用服务与 WebSocket 房间中心。
type Handler struct {
	service *chatapp.Service
	hub     *wschat.Hub
}

// NewHandler 构造私信 HTTP 处理器。
func NewHandler(service *chatapp.Service, hub *wschat.Hub) *Handler {
	return &Handler{service: service, hub: hub}
}
