// Package dynamic 是动态上下文的 HTTP 处理器（原 api/dynamic_api）。
//
// 处理器只做参数绑定、身份提取与响应渲染；业务编排全部在 application 层。
package dynamic

import app "f_ccg_video_back_ddd/internal/application/dynamic"

// Handler 动态相关接口处理器。
type Handler struct {
	dynamics *app.Service
	feed     *app.FeedService
	follows  *app.FollowService
	likes    *app.LikeService
	reads    *app.ReadService
	admin    *app.AdminService
}

// NewHandler 构造动态处理器。
func NewHandler(
	dynamics *app.Service,
	feed *app.FeedService,
	follows *app.FollowService,
	likes *app.LikeService,
	reads *app.ReadService,
	admin *app.AdminService,
) *Handler {
	return &Handler{
		dynamics: dynamics,
		feed:     feed,
		follows:  follows,
		likes:    likes,
		reads:    reads,
		admin:    admin,
	}
}
