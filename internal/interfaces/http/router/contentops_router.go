package router

import (
	contentopshandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/contentops"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/gin-gonic/gin"
)

// registerContentOpsCommonRoutes 装配 /v1/common 下的 site 与 banner 路由，按 site、banner 顺序注册。
func registerContentOpsCommonRoutes(
	g *gin.RouterGroup,
	mw *middlewares.Middlewares,
	h *contentopshandler.Handler,
) {
	siteRouter := g.Group("site")
	{
		siteRouter.GET("qq-url", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), h.QQUrlView)
		siteRouter.GET("google-url", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), h.GoogleUrlView)
		siteRouter.GET("github-url", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout),
			middlewares.BindQueryParams[contentopshandler.GitHubURLReq], h.GitHubUrlView)
		siteRouter.GET("linuxdo-url", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), h.LinuxDoUrlView)
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

	bannerGroup := g.Group("banner")
	{
		bannerGroup.GET("list", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout),
			middlewares.BindQueryParams[contentopshandler.BannerListReq], h.ListView)
	}
}

// registerContentOpsAdminRoutes 装配 /v1/admin 下的 banner 与 site 路由，按 banner、site 顺序注册。
func registerContentOpsAdminRoutes(
	g *gin.RouterGroup,
	mw *middlewares.Middlewares,
	h *contentopshandler.Handler,
) {
	bannerGroup := g.Group("/banner")
	bannerGroup.Use(middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.AuthAll())
	{
		bannerGroup.POST("", middlewares.JsonBindParams[contentopshandler.AdminCreateBannerReq], h.AdminCreateView)
		bannerGroup.PUT("", middlewares.JsonBindParams[contentopshandler.AdminUpdateBannerReq], h.AdminUpdateView)
		bannerGroup.DELETE("", middlewares.JsonBindParams[contentopshandler.AdminDeleteBannerReq], h.AdminDeleteView)
	}

	listGroup := g.Group("/banners")
	listGroup.Use(middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.AuthAll())
	{
		listGroup.GET("", middlewares.BindQueryParams[contentopshandler.AdminBannerListReq], h.AdminListView)
	}

	siteGroup := g.Group("/site")
	siteGroup.Use(middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.AuthAll())
	{
		siteGroup.GET("config", h.AdminConfigItemView) // ?name=
		siteGroup.PUT("config", h.AdminUpdateConfigItemView)
		siteGroup.GET("stats", h.AdminStatsView)
	}
}
