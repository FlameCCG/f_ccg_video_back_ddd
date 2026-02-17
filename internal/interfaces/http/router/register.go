package router

import "github.com/gin-gonic/gin"

// registerCommonRoutes 装配 /v1/common 下的全部路由。
//
// 注册顺序为 user → captcha → site → ai → banner → social → video → comment → chat → notification → dynamic。
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
// 注册顺序为 rbac → user → banner → video → comment → site → notification → dynamic。
// banner 与 site 的管理接口由内容运营路由注册函数共同装配。
func (r *Router) registerAdminRoutes(group *gin.RouterGroup) {
	registerRbacRoutes(group, r.mw, r.rbac)
	registerIdentityAdminRoutes(group, r.mw, r.identityAdmin)
	registerContentOpsAdminRoutes(group, r.mw, r.contentOps)
	registerVideoAdminRoutes(group, r.mw, r.videoAdmin)
	registerCommentAdminRoutes(group, r.mw, r.comment)
	registerNotificationAdminRoutes(group, r.mw, r.notification)
	registerDynamicAdminRoutes(group, r.mw, r.dynamic)
}
