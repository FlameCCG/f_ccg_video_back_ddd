// Package social 是社交关系上下文的 HTTP 处理器。
//
// 处理器只做参数解析、身份提取与响应渲染；隐私判定与写侧编排在 application/domain。
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

// viewerID 读取当前登录用户 ID 的 int 值；未登录或解析失败返回 0。
func viewerID(c *gin.Context) int {
	uid, ok := middlewares.GetUID(c)
	if !ok {
		return 0
	}

	return int(uid)
}
