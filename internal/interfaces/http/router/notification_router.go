package router

import (
	notificationhandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/notification"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/gin-gonic/gin"
)

// registerNotificationCommonRoutes 装配 /v1/common/notification 路由
// （逐行对应原 router/notification_router.go）。
func registerNotificationCommonRoutes(
	group *gin.RouterGroup,
	mw *middlewares.Middlewares,
	handler *notificationhandler.Handler,
) {
	notificationGroup := group.Group("/notification")
	notificationGroup.Use(middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth())
	{
		notificationGroup.GET("", middlewares.BindQueryParams[notificationhandler.ListNotificationReq], handler.List)
		notificationGroup.GET("/counts", handler.Counts)
		notificationGroup.PUT("/read", middlewares.JSONBindParams[notificationhandler.MarkReadReq], handler.MarkRead)
		notificationGroup.DELETE("", middlewares.JSONBindParams[notificationhandler.DeleteNotificationReq], handler.Delete)
	}
}

// registerNotificationAdminRoutes 装配 /v1/admin/notification 路由
// （逐行对应原 router/notification_admin_router.go）。
func registerNotificationAdminRoutes(
	group *gin.RouterGroup,
	mw *middlewares.Middlewares,
	handler *notificationhandler.Handler,
) {
	notificationGroup := group.Group("/notification")
	notificationGroup.Use(middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.AuthAll())
	{
		notificationGroup.POST("", middlewares.JSONBindParams[notificationhandler.CreateNotificationReq], handler.Create)
		notificationGroup.PUT("", middlewares.JSONBindParams[notificationhandler.AdminUpdateReq], handler.AdminUpdate)
		notificationGroup.GET("", middlewares.BindQueryParams[notificationhandler.AdminListReq], handler.AdminList)
		notificationGroup.DELETE("", middlewares.JSONBindParams[notificationhandler.DeleteNotificationReq], handler.AdminDelete)
	}
}
