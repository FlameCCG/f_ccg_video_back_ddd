package router

import "github.com/gin-gonic/gin"

// registerCommonRoutes 装配 /v1/common 下的全部路由。
//
// 调用顺序与原 router/enter.go 的 commonGroup 完全一致：
// user → captcha → site → ai → banner → social → video → comment → chat → notification → dynamic。
// （原 initSiteRouter / initBannerRouter 在本仓库合并为 content-ops 上下文，
// 故按 site、banner 的原有相对位置分别调用其 common 注册函数。）
func (r *Router) registerCommonRoutes(group *gin.RouterGroup) {
	registerIdentityCommonRoutes(group, r.mw, r.identity)
	registerCaptchaRoutes(group, r.mw, r.captcha)
	registerContentOpsCommonRoutes(group, r.mw, r.contentOps)
	registerAIRoutes(group, r.mw, r.ai)
	registerSocialRoutes(group, r.mw, r.social)
	registerVideoCommonRoutes(group, r.mw, r.video)
	registerCommentCommonRoutes(group, r.mw, r.comment)
	registerChatRoutes(group, r.mw, r.chat)
	registerNotificationCommonRoutes(group, r.mw, r.notification)
	registerDynamicCommonRoutes(group, r.mw, r.dynamic)
}

// registerAdminRoutes 装配 /v1/admin 下的全部路由。
//
// 调用顺序与原 router/enter.go 的 adminGroup 完全一致：
// rbac → user → banner → video → comment → site → notification → dynamic。
// （banner 与 site 的管理端路由同属 content-ops 上下文，由同一个注册函数装配。）
func (r *Router) registerAdminRoutes(group *gin.RouterGroup) {
	registerRbacRoutes(group, r.mw, r.rbac)
	registerIdentityAdminRoutes(group, r.mw, r.identityAdmin)
	registerContentOpsAdminRoutes(group, r.mw, r.contentOps)
	registerVideoAdminRoutes(group, r.mw, r.videoAdmin)
	registerCommentAdminRoutes(group, r.mw, r.comment)
	registerNotificationAdminRoutes(group, r.mw, r.notification)
	registerDynamicAdminRoutes(group, r.mw, r.dynamic)
}
