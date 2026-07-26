// Package notification 提供通知相关 HTTP 接口（原 api/notification_api）。
package notification

import (
	app "f_ccg_video_back_ddd/internal/application/notification"
)

// Handler 通知相关接口。
type Handler struct {
	svc *app.Service
}

// NewHandler 构造通知处理器。
func NewHandler(svc *app.Service) *Handler {
	return &Handler{svc: svc}
}
