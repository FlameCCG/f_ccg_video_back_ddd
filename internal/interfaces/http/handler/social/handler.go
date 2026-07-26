// Package social 是社交关系上下文的 HTTP 处理器（原 api/social_api）。
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
