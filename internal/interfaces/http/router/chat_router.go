package router

import (
	chathandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/chat"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/gin-gonic/gin"
)

// registerChatRoutes 装配私信路由（原 router/chat_router.go，method + path 逐行一致）。
func registerChatRoutes(g *gin.RouterGroup, mw *middlewares.Middlewares, h *chathandler.Handler) {
	chatGroup := g.Group("/chat")
	{
		chatGroup.GET("/ws",
			mw.JwtAuthWithQueryToken("token"),
			h.WSView,
		)
	}

	authGroup := chatGroup.Group("")
	authGroup.Use(middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth())
	{
		authGroup.GET("/conversations",
			middlewares.BindQueryParams[chathandler.ConversationListReq], h.ConversationListView)
		authGroup.POST("/conversations",
			middlewares.JSONBindParams[chathandler.CreateConversationReq], h.CreateConversationView)
		authGroup.DELETE("/conversations",
			middlewares.JSONBindParams[chathandler.DeleteConversationReq], h.DeleteConversationView)
		authGroup.GET("/messages",
			middlewares.BindQueryParams[chathandler.MessageListReq], h.MessageListView)
		authGroup.PUT("/read",
			middlewares.JSONBindParams[chathandler.MarkReadReq], h.MarkReadView)
	}
}
