package router

import (
	identityhandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/gin-gonic/gin"
)

// registerIdentityCommonRoutes 装配 /v1/common/user 路由（逐行对应原 router/user_router.go）。
func registerIdentityCommonRoutes(
	g *gin.RouterGroup,
	mw *middlewares.Middlewares,
	h *identityhandler.Handler,
) {
	userGroup := g.Group("user")

	// 公开的用户信息（无需认证）
	userGroup.GET("/search", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtOptional(), middlewares.BindQueryParams[identityhandler.UserSearchReq], h.SearchView)
	userGroup.GET("/banner/defaults", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtOptional(), h.ListDefaultUserBannersView)
	userGroup.GET("/detail", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtOptional(), h.UserDetailView) // 获取指定用户的公开信息 ?id=

	userVideoGroup := userGroup.Group("video")
	userVideoGroup.Use(middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.JwtOptional())
	{
		userVideoGroup.GET("/list", middlewares.BindQueryParams[identityhandler.UserVideosReq], h.UserVideosView)
		userVideoGroup.GET("/recent/liked", h.RecentLikedView)
		userVideoGroup.GET("/recent/coined", h.RecentCoinedView)
		userVideoGroup.DELETE("/delete", middlewares.JsonBindParams[identityhandler.DeleteVideoReq], mw.JwtAuth(), h.DeleteVideoView)
		userVideoGroup.POST("/tag", mw.JwtAuth(), mw.VideoSubmissionCheck(), mw.UserBanCheck(), middlewares.JsonBindParams[identityhandler.CreateTagReq], h.CreateTag)
	}

	// @ 联想（需登录）
	userGroup.GET("/mention/suggest",
		middlewares.RequestTimeout(ctxutil.RequestFastTimeout),
		mw.JwtAuth(),
		middlewares.BindQueryParams[identityhandler.MentionSuggestReq],
		h.MentionSuggestView)

	// 创作中心数据分析（需登录）
	creatorGroup := userGroup.Group("creator")
	creatorGroup.Use(middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth())
	{
		creatorGroup.GET("/analytics", middlewares.BindQueryParams[identityhandler.CreatorAnalyticsReq], h.CreatorAnalyticsView)
		creatorGroup.GET("/overview", h.CreatorOverviewView)
	}

	// 记录功能（需登录）
	recordGroup := userGroup.Group("record")
	recordGroup.Use(middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth())
	{
		recordGroup.GET("/login-ip", middlewares.BindQueryParams[identityhandler.LoginIPRecordListReq], h.LoginIPRecordListView)
		recordGroup.GET("/exp", middlewares.BindQueryParams[identityhandler.ExpRecordListReq], h.ExpRecordListView)
		recordGroup.GET("/coin", middlewares.BindQueryParams[identityhandler.CoinRecordListReq], h.CoinRecordListView)
	}

	// 注册相关
	registerGroup := userGroup.Group("register")
	{
		registerGroup.POST("/email",
			middlewares.RequestTimeout(ctxutil.RequestSlowTimeout),
			mw.EmailCaptchaVerify(),
			middlewares.JsonBindParams[identityhandler.RegisterReq],
			h.EmailRegisterView,
		)
	}

	// 登录相关
	loginGroup := userGroup.Group("login")
	{
		loginGroup.POST("/pwd",
			middlewares.RequestTimeout(ctxutil.RequestSlowTimeout),
			mw.LoginRateLimit(),
			mw.InteractiveCaptchaVerify("login"),
			middlewares.JsonBindParams[identityhandler.PwdLoginReq],
			h.PwdLoginView,
		)
		loginGroup.POST("/qq", middlewares.RequestTimeout(ctxutil.RequestOAuthTimeout), mw.LoginRateLimit(), middlewares.JsonBindParams[identityhandler.QQLoginReq], h.QQLoginView)
		loginGroup.POST("/google", middlewares.RequestTimeout(ctxutil.RequestOAuthTimeout), mw.LoginRateLimit(), middlewares.JsonBindParams[identityhandler.GoogleLoginReq], h.GoogleLoginView)
		loginGroup.POST("/github", middlewares.RequestTimeout(ctxutil.RequestOAuthTimeout), mw.LoginRateLimit(), middlewares.JsonBindParams[identityhandler.GitHubLoginReq], h.GitHubLoginView)
		loginGroup.POST("/linuxdo", middlewares.RequestTimeout(ctxutil.RequestOAuthTimeout), mw.LoginRateLimit(), middlewares.JsonBindParams[identityhandler.LinuxDoLoginReq], h.LinuxDoLoginView)
		loginGroup.POST("/x", middlewares.RequestTimeout(ctxutil.RequestOAuthTimeout), mw.LoginRateLimit(), middlewares.JsonBindParams[identityhandler.XLoginReq], h.XLoginView)
		loginGroup.POST("/refresh", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), middlewares.JsonBindParams[identityhandler.RefreshTokenReq], h.RefreshTokenView)
	}

	// 用户信息模块
	infoGroup := userGroup.Group("info")
	infoGroup.Use(middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth())
	{
		infoGroup.GET("", h.UserInfoView)                                                                           // 获取当前用户信息
		infoGroup.PUT("", middlewares.JsonBindParams[identityhandler.UpdateUserInfoReq], h.UpdateUserInfoView)      // 更新当前用户信息
		infoGroup.GET("/conf", h.GetUserConfView)                                                                   // 获取用户配置
		infoGroup.PUT("/conf", middlewares.JsonBindParams[identityhandler.UpdateUserConfReq], h.UpdateUserConfView) // 更新用户配置
	}

	userGroup.POST("/banner/upload", middlewares.RequestTimeout(ctxutil.RequestUploadTimeout), mw.JwtAuth(), h.UploadBannerView)

	// 邮箱绑定（需登录）
	userGroup.POST("/email/bind",
		middlewares.RequestTimeout(ctxutil.RequestSlowTimeout),
		mw.JwtAuth(),
		mw.EmailCaptchaVerifyRequired(),
		middlewares.JsonBindParams[identityhandler.BindEmailReq],
		h.BindEmailView,
	)

	// 密码找回/修改
	userGroup.POST("/password/reset",
		middlewares.RequestTimeout(ctxutil.RequestSlowTimeout),
		mw.EmailCaptchaVerifyRequired(),
		middlewares.JsonBindParams[identityhandler.ResetPasswordReq],
		h.ResetPasswordView,
	)

	userGroup.PUT("/password/change",
		middlewares.RequestTimeout(ctxutil.RequestSlowTimeout),
		mw.JwtAuth(),
		middlewares.JsonBindParams[identityhandler.ChangePasswordReq],
		h.ChangePasswordView,
	)
}

// registerIdentityAdminRoutes 装配 /v1/admin/user 路由（逐行对应原 router/user_admin_router.go）。
func registerIdentityAdminRoutes(
	g *gin.RouterGroup,
	mw *middlewares.Middlewares,
	h *identityhandler.AdminHandler,
) {
	loginGroup := g.Group("/user/login")
	{
		loginGroup.POST("pwd",
			middlewares.RequestTimeout(ctxutil.RequestSlowTimeout),
			mw.LoginRateLimit(),
			mw.InteractiveCaptchaVerify("login"),
			middlewares.JsonBindParams[identityhandler.AdminPwdLoginReq],
			h.AdminPwdLoginView,
		)
	}

	userGroup := g.Group("/user")
	userGroup.Use(middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.AuthAll())
	{
		userGroup.GET("list", middlewares.BindQueryParams[identityhandler.AdminUserListReq], h.AdminUserListView)
		userGroup.PUT("role/switch", middlewares.JsonBindParams[identityhandler.SwitchRoleReq], h.SwitchRoleView)
		userGroup.GET("info", h.AdminUserInfoView)
		userGroup.PUT("info", middlewares.JsonBindParams[identityhandler.AdminUpdateUserInfoReq], h.AdminUpdateUserInfoView)
		userGroup.PUT("banner", middlewares.JsonBindParams[identityhandler.AdminSetUserBannerReq], h.AdminSetUserBannerView)
		userGroup.PUT("banner/defaults", middlewares.JsonBindParams[identityhandler.AdminUpdateUserBannerListReq], h.AdminUpdateUserBannerListView)
		userGroup.POST("ban", middlewares.JsonBindParams[identityhandler.AdminUserBanReq], h.AdminBanView)
		userGroup.GET("ban/records", middlewares.BindQueryParams[identityhandler.AdminBanRecordListReq], h.AdminBanRecordListView)
	}
}

// registerCaptchaRoutes 装配 /v1/common/captcha 路由（逐行对应原 router/captcha_router.go）。
func registerCaptchaRoutes(
	g *gin.RouterGroup,
	mw *middlewares.Middlewares,
	h *identityhandler.CaptchaHandler,
) {
	captchaGroup := g.Group("captcha")
	{
		captchaGroup.POST("email",
			middlewares.RequestTimeout(ctxutil.RequestLongTimeout),
			mw.InteractiveCaptchaVerify("register"),
			mw.GraphicsTextCaptchaMiddleware(),
			middlewares.BindParams[identityhandler.EmailCaptchaRequest],
			h.EmailCaptchaView,
		)
		captchaGroup.GET("graphics-text", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), middlewares.BindParams[identityhandler.GraphicsCaptchaRequest], h.GraphicsTextCaptchaView)
		captchaGroup.GET("click", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), h.ClickCaptchaView)
		captchaGroup.GET("slide", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), h.SlideCaptchaView)
		captchaGroup.GET("type", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), h.CaptchaTypeView)
	}
}
