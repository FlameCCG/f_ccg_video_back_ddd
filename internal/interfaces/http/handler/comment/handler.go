// Package comment 把评论创建、回复、点赞、置顶和管理查询的 HTTP 请求翻译为应用用例调用。
//
// handler.go 声明显式依赖，其他文件按入口拆分；参数结构体保留 Gin 绑定与校验标签。
// 从请求取得参数和经过鉴权的身份，构造应用参数后传 c.Request.Context()，最后映射错误和响应。
// gin.Context 止步于接口层，领域规则、事务、缓存与跨上下文调用由 application 编排。
// 具体入口与权限链到 internal/interfaces/http/router 中查看，业务允许条件还需由应用与领域判断。
// JSON 字段、状态码和响应文案属于对外契约，新增注释不应顺带调整这些行为。
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
