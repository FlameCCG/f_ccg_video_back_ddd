package router

import (
	contentopshandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/contentops"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/gin-gonic/gin"
)

// registerContentOpsCommonRoutes 装配 /v1/common 下的 banner 与 site 路由。
//
// 分组、中间件顺序与 path 与原 router/site_router.go、router/banner_router.go 逐行一致；
// 两组的注册顺序沿用原 router/enter.go 中 commonGroup 的调用顺序（site 先于 banner）。
func registerContentOpsCommonRoutes(
	g *gin.RouterGroup,
	mw *middlewares.Middlewares,
	h *contentopshandler.Handler,
) {
	// 原 router.initSiteRouter
	siteRouter := g.Group("site")
	{
		siteRouter.GET("qq-url", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), h.QQUrlView)
		siteRouter.GET("google-url", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), h.GoogleURLView)
		siteRouter.GET("github-url", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout),
			middlewares.BindQueryParams[contentopshandler.GitHubURLReq], h.GitHubURLView)
		siteRouter.GET("linuxdo-url", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout),
			middlewares.BindQueryParams[contentopshandler.LinuxDoURLReq], h.LinuxDoURLView)
		siteRouter.GET("x-url", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout),
			middlewares.BindQueryParams[contentopshandler.XURLReq], h.XUrlView)
		siteRouter.GET("config", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), h.CommonConfigView)

		siteStatGroup := siteRouter.Group("stat")
		siteStatGroup.Use(middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtOptional())
		{
			siteStatGroup.POST("touch", h.StatTouchView)
			siteStatGroup.POST("heartbeat", h.StatHeartbeatView)
		}
	}

	// 原 router.initBannerRouter
	bannerGroup := g.Group("banner")
	{
		bannerGroup.GET("list", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout),
			middlewares.BindQueryParams[contentopshandler.BannerListReq], h.ListView)
	}
}

// registerContentOpsAdminRoutes 装配 /v1/admin 下的 banner 与 site 路由。
//
// 分组、中间件顺序与 path 与原 router/banner_admin_router.go、router/site_admin_router.go 逐行一致；
// 两组的注册顺序沿用原 router/enter.go 中 adminGroup 的调用顺序（banner 先于 site）。
func registerContentOpsAdminRoutes(
	g *gin.RouterGroup,
	mw *middlewares.Middlewares,
	h *contentopshandler.Handler,
) {
	// 原 router.initBannerAdminRouter
	bannerGroup := g.Group("/banner")
	bannerGroup.Use(middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.AuthAll())
	{
		bannerGroup.POST("", middlewares.JSONBindParams[contentopshandler.AdminCreateBannerReq], h.AdminCreateView)
		bannerGroup.PUT("", middlewares.JSONBindParams[contentopshandler.AdminUpdateBannerReq], h.AdminUpdateView)
		bannerGroup.DELETE("", middlewares.JSONBindParams[contentopshandler.AdminDeleteBannerReq], h.AdminDeleteView)
	}

	listGroup := g.Group("/banners")
	listGroup.Use(middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.AuthAll())
	{
		listGroup.GET("", middlewares.BindQueryParams[contentopshandler.AdminBannerListReq], h.AdminListView)
	}

	// 原 router.initSiteAdminRouter
	siteGroup := g.Group("/site")
	siteGroup.Use(middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.AuthAll())
	{
		siteGroup.GET("config", h.AdminConfigItemView) // ?name=
		siteGroup.PUT("config", h.AdminUpdateConfigItemView)
		siteGroup.GET("stats", h.AdminStatsView)
	}
}
