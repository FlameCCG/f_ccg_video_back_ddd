// Package comment 提供评论 HTTP 处理器。
//
// gin.Context 止于接口层，应用层仅接收 context.Context 与 DTO。
package comment

import (
	app "f_ccg_video_back_ddd/internal/application/comment"
	"f_ccg_video_back_ddd/pkg/utils/ipUtils"
)

// Handler 评论 HTTP 处理器。
type Handler struct {
	svc      *app.Service
	resolver ipUtils.RegionResolver
}

// NewHandler 构造评论处理器，resolver 用于解析客户端 IP 归属地。
func NewHandler(svc *app.Service, resolver ipUtils.RegionResolver) *Handler {
	return &Handler{svc: svc, resolver: resolver}
}
