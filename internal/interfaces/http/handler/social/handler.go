// Package social 把关注、取关和关注/粉丝列表的 HTTP 请求翻译为应用用例调用。
//
// handler.go 声明显式依赖，其他文件按入口拆分；参数结构体保留 Gin 绑定与校验标签。
// 从请求取得参数和经过鉴权的身份，构造应用参数后传 c.Request.Context()，最后映射错误和响应。
// gin.Context 止步于接口层，领域规则、事务、缓存与跨上下文调用由 application 编排。
// 具体入口与权限链到 internal/interfaces/http/router 中查看，业务允许条件还需由应用与领域判断。
// JSON 字段、状态码和响应文案属于对外契约，新增注释不应顺带调整这些行为。
package social

import (
	app "f_ccg_video_back_ddd/internal/application/social"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"

	"github.com/gin-gonic/gin"
)

// Handler 关注/粉丝等社交关系接口处理器。
type Handler struct {
	focus     *app.FocusService
	relations *app.RelationService
}

// NewHandler 构造社交关系处理器。
func NewHandler(focus *app.FocusService, relations *app.RelationService) *Handler {
	return &Handler{focus: focus, relations: relations}
}

// viewerID 读取当前登录用户 ID 的 int 形式；未登录返回 0。
//
// 原实现用 strconv.Atoi(c.GetString("uid")) 取值并与 query 中的 id 做 int 比较，
// 此处保持同样的 int 语义（未登录/解析失败均为 0）。
func viewerID(c *gin.Context) int {
	uid, ok := middlewares.GetUID(c)
	if !ok {
		return 0
	}

	return int(uid)
}
