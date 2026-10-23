// Package dynamic 把动态发布、列表、关注流、点赞、置顶和已读的 HTTP 请求翻译为应用用例调用。
//
// handler.go 声明显式依赖，其他文件按入口拆分；参数结构体保留 Gin 绑定与校验标签。
// 从请求取得参数和经过鉴权的身份，构造应用参数后传 c.Request.Context()，最后映射错误和响应。
// gin.Context 止步于接口层，领域规则、事务、缓存与跨上下文调用由 application 编排。
// 具体入口与权限链到 internal/interfaces/http/router 中查看，业务允许条件还需由应用与领域判断。
// JSON 字段、状态码和响应文案属于对外契约，新增注释不应顺带调整这些行为。
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
