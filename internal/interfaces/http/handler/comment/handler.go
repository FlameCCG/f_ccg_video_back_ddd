// Package comment 是评论上下文的 HTTP 处理器（原 api/comment_api）。
//
// 方法名与原 CommentAPI 的导出方法完全一致；请求结构体字段与 tag 逐字照搬。
// gin.Context 止步于本包：向应用层只传 context.Context 与 DTO。
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

// NewHandler 构造评论处理器。
//
// resolver 用于把客户端 IP 解析为归属地（原 ipUtils 直读 global.IPSearcher）。
func NewHandler(svc *app.Service, resolver ipUtils.RegionResolver) *Handler {
	return &Handler{svc: svc, resolver: resolver}
}
