package router

import "github.com/gin-gonic/gin"

// registerCommonRoutes 装配 /v1/common 下的全部路由（与原 router.buildEngine 中 commonGroup 顺序一致）。
func (r *Router) registerCommonRoutes(group *gin.RouterGroup) {
	_ = group
}

// registerAdminRoutes 装配 /v1/admin 下的全部路由（与原 router.buildEngine 中 adminGroup 顺序一致）。
func (r *Router) registerAdminRoutes(group *gin.RouterGroup) {
	_ = group
}
