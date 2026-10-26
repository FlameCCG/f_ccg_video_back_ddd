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
		dynamicGroup.POST("create", middlewares.JSONBindParams[dynamichandler.CreateDynamicReq], h.CreateView)
		dynamicGroup.DELETE("delete", middlewares.JSONBindParams[dynamichandler.DeleteDynamicReq], h.DeleteView)
		dynamicGroup.PUT("pin", middlewares.JSONBindParams[dynamichandler.PinDynamicReq], h.PinView)
		dynamicGroup.POST("like", middlewares.JSONBindParams[dynamichandler.LikeReq], h.LikeView)

		dynamicGroup.GET("list", middlewares.BindQueryParams[dynamichandler.ListReq], h.ListView)
		dynamicGroup.GET("follows", middlewares.BindQueryParams[dynamichandler.FollowListReq], h.FollowListView)
		dynamicGroup.GET("counts", h.CountsView)
		dynamicGroup.POST("read", middlewares.JSONBindParams[dynamichandler.ReadReq], h.ReadView)
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
		dynamicGroup.DELETE("", middlewares.JSONBindParams[dynamichandler.AdminDeleteDynamicReq], h.AdminDeleteView)
	}
}
