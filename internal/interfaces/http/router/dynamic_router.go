package router

import (
	dynamichandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/dynamic"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/gin-gonic/gin"
)

// registerDynamicCommonRoutes 装配 /v1/common/dynamic 下的路由（原 router/dynamic_router.go）。
func registerDynamicCommonRoutes(
	g *gin.RouterGroup,
	mw *middlewares.Middlewares,
	h *dynamichandler.Handler,
) {
	dynamicGroup := g.Group("/dynamic")
	dynamicGroup.Use(middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.JwtAuth())
	{
		dynamicGroup.POST("create", middlewares.JsonBindParams[dynamichandler.CreateDynamicReq], h.CreateView)
		dynamicGroup.DELETE("delete", middlewares.JsonBindParams[dynamichandler.DeleteDynamicReq], h.DeleteView)
		dynamicGroup.PUT("pin", middlewares.JsonBindParams[dynamichandler.PinDynamicReq], h.PinView)
		dynamicGroup.POST("like", middlewares.JsonBindParams[dynamichandler.DynamicLikeReq], h.LikeView)

		dynamicGroup.GET("list", middlewares.BindQueryParams[dynamichandler.DynamicListReq], h.ListView)
		dynamicGroup.GET("follows", middlewares.BindQueryParams[dynamichandler.DynamicFollowListReq], h.FollowListView)
		dynamicGroup.GET("counts", h.CountsView)
		dynamicGroup.POST("read", middlewares.JsonBindParams[dynamichandler.DynamicReadReq], h.ReadView)
	}
}

// registerDynamicAdminRoutes 装配 /v1/admin/dynamic 下的路由（原 router/dynamic_admin_router.go）。
func registerDynamicAdminRoutes(
	g *gin.RouterGroup,
	mw *middlewares.Middlewares,
	h *dynamichandler.Handler,
) {
	dynamicGroup := g.Group("/dynamic")
	dynamicGroup.Use(middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.AuthAll())
	{
		dynamicGroup.GET("list", middlewares.BindQueryParams[dynamichandler.AdminDynamicListReq], h.AdminListView)
		dynamicGroup.DELETE("", middlewares.JsonBindParams[dynamichandler.AdminDeleteDynamicReq], h.AdminDeleteView)
	}
}
