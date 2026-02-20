// Package chat 是私信上下文的 HTTP 处理器。
package chat

import (
	chatapp "f_ccg_video_back_ddd/internal/application/chat"
	wschat "f_ccg_video_back_ddd/internal/interfaces/ws/chat"
)

// Handler 是私信 HTTP 处理器，通过 Wire 注入应用服务与 WebSocket 房间中心。
type Handler struct {
	service *chatapp.Service
	hub     *wschat.Hub
}

// NewHandler 构造私信 HTTP 处理器。
func NewHandler(service *chatapp.Service, hub *wschat.Hub) *Handler {
	return &Handler{service: service, hub: hub}
}
