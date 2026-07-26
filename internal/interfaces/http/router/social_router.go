package router

import (
	socialhandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/social"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/gin-gonic/gin"
)

// registerSocialRoutes 装配 /v1/common/social 下的路由（原 router/social_router.go）。
func registerSocialRoutes(
	g *gin.RouterGroup,
	mw *middlewares.Middlewares,
	h *socialhandler.Handler,
) {
	socialRouter := g.Group("social")
	socialGroup := socialRouter.Group("")
	socialGroup.Use(middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth())
	{
		socialGroup.POST("/focus", middlewares.JsonBindParams[socialhandler.FocusUserReq], h.FocusUserView)       // 关注用户
		socialGroup.DELETE("/focus", middlewares.JsonBindParams[socialhandler.UnfocusUserReq], h.UnfocusUserView) // 取消关注
		socialGroup.GET("/focus", h.FocusListView)                                                                // 获取用户关注列表 ?id=
		socialGroup.GET("/fans", h.FansListView)                                                                  // 获取用户粉丝列表 ?id=
		socialGroup.GET("/relation", h.FocusStatusView)                                                           // 查询与指定用户的关注关系 ?id=
	}
}
