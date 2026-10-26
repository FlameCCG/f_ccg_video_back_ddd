package router

import (
	aiapp "f_ccg_video_back_ddd/internal/application/ai"
	aihandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/ai"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/gin-gonic/gin"
)

// registerAIRoutes 装配 AI 路由（原 router/ai_router.go，method + path 逐行一致）。
func registerAIRoutes(g *gin.RouterGroup, mw *middlewares.Middlewares, h *aihandler.Handler) {
	aiPublicGroup := g.Group("ai")
	aiPublicGroup.Use(middlewares.RequestTimeout(ctxutil.RequestAITimeout))
	{
		// 模型/思考选项不含密钥，允许未登录拉取以便前台渲染下拉
		aiPublicGroup.GET("options", h.OptionsView)
		aiPublicGroup.GET(
			"assets",
			middlewares.BindQueryParams[aihandler.AssetProxyReq],
			h.AssetProxyView,
		)
	}

	aiStreamGroup := g.Group("ai")
	aiStreamGroup.Use(middlewares.RequestTimeout(ctxutil.RequestAITimeout), mw.JwtAuthWithQueryToken("token"), mw.UserBanCheck(), mw.AIGenerationLimit())
	{
		aiStreamGroup.GET(
			"responses/stream",
			middlewares.BindQueryParams[aihandler.ResponsesStreamReq],
			h.ResponsesStreamView,
		)
	}

	aiGroup := g.Group("ai")
	aiGroup.Use(middlewares.RequestTimeout(ctxutil.RequestAITimeout), mw.JwtAuth())
	{
		aiGroup.POST(
			"responses",
			mw.UserBanCheck(), mw.AIGenerationLimit(),
			middlewares.JSONBindParams[aiapp.ResponsesRequest],
			h.ResponsesView,
		)
		aiGroup.POST(
			"images/generations",
			mw.UserBanCheck(), mw.AIGenerationLimit(),
			middlewares.JSONBindParams[aiapp.ImageGenerationRequest],
			h.GenerateImageView,
		)
		aiGroup.POST(
			"images/edits",
			mw.UserBanCheck(), mw.AIGenerationLimit(),
			middlewares.JSONBindParams[aiapp.ImageEditRequest],
			h.EditImageView,
		)
		aiGroup.POST(
			"videos/generations",
			mw.UserBanCheck(), mw.AIGenerationLimit(),
			middlewares.JSONBindParams[aiapp.VideoGenerationRequest],
			h.GenerateVideoView,
		)
		aiGroup.GET("videos/status", h.VideoStatusView) // ?requestID=
	}
}
