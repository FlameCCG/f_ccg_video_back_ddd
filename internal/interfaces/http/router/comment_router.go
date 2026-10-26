package router

import (
	commenthandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/comment"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/gin-gonic/gin"
)

// registerCommentCommonRoutes 装配 /v1/common/comment 路由（原 router/comment_router.go）。
func registerCommentCommonRoutes(
	g *gin.RouterGroup,
	mw *middlewares.Middlewares,
	h *commenthandler.Handler,
) {
	commentGroup := g.Group("/comment")
	{
		// 公开接口（可选登录获取点赞状态）
		commentGroup.GET("list", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtOptional(), middlewares.BindQueryParams[commenthandler.ListReq], h.ListView)
		commentGroup.GET("replies", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtOptional(), middlewares.BindQueryParams[commenthandler.RepliesReq], h.RepliesView)
	}

	authGroup := commentGroup.Group("")
	authGroup.Use(middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.JwtAuth())
	{
		authGroup.POST("create",
			mw.UserBanCheck(),
			middlewares.JsonBindParams[commenthandler.CreateCommentReq],
			h.CreateView,
		)
		authGroup.DELETE("delete", middlewares.JsonBindParams[commenthandler.DeleteCommentReq], h.DeleteView)
		authGroup.PUT("pin", middlewares.JsonBindParams[commenthandler.PinReq], h.PinView)
		authGroup.POST("like", middlewares.JsonBindParams[commenthandler.LikeReq], h.LikeView)
		authGroup.GET("creator/list", middlewares.BindQueryParams[commenthandler.CreatorCommentListReq], h.CreatorListView)
	}
}

// registerCommentAdminRoutes 装配 /v1/admin/comment 路由（原 router/comment_admin_router.go）。
func registerCommentAdminRoutes(
	g *gin.RouterGroup,
	mw *middlewares.Middlewares,
	h *commenthandler.Handler,
) {
	commentGroup := g.Group("/comment")
	commentGroup.Use(middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.AuthAll())
	{
		commentGroup.GET("list", middlewares.BindQueryParams[commenthandler.AdminCommentListReq], h.AdminListView)
		commentGroup.DELETE("", middlewares.JsonBindParams[commenthandler.AdminDeleteCommentReq], h.AdminDeleteView)
	}
}
