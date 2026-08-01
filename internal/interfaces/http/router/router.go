// Package router 汇总 HTTP 路由装配（原 router 包）。
//
// 路由的 method + path 必须与原项目完全一致：/v1/common/*、/v1/admin/*、静态目录 /uploads。
package router

import (
	aihandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/ai"
	chathandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/chat"
	commenthandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/comment"
	contentopshandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/contentops"
	dynamichandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/dynamic"
	identityhandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/identity"
	notificationhandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/notification"
	rbachandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/rbac"
	socialhandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/social"
	videohandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/video"

	"f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/validate"

	"github.com/gin-gonic/gin"
)

// staticUploadsRoute 本地存储对外暴露的静态目录（与原项目一致）。
const (
	staticUploadsRoute = "/uploads"
	staticUploadsDir   = "uploads"
)

// Router 持有全部 HTTP 处理器与所需中间件依赖，负责装配 gin 引擎。
//
// 依赖全部由 wire 在 bootstrap 注入；本类型只做路由声明，不含业务逻辑。
type Router struct {
	cfgProvider *config.Provider
	mw          *middlewares.Middlewares
	validator   *validate.Translator

	identity      *identityhandler.Handler
	identityAdmin *identityhandler.AdminHandler
	captcha       *identityhandler.CaptchaHandler
	contentOps    *contentopshandler.Handler
	ai            *aihandler.Handler
	social        *socialhandler.Handler
	video         *videohandler.Handler
	videoAdmin    *videohandler.AdminHandler
	comment       *commenthandler.Handler
	chat          *chathandler.Handler
	notification  *notificationhandler.Handler
	dynamic       *dynamichandler.Handler
	rbac          *rbachandler.Handler
}

// New 构造路由装配器。
//
// 形参顺序对应原 router/enter.go 中 commonGroup / adminGroup 的装配顺序。
func New(
	cfgProvider *config.Provider,
	mw *middlewares.Middlewares,
	validator *validate.Translator,
	identity *identityhandler.Handler,
	identityAdmin *identityhandler.AdminHandler,
	captcha *identityhandler.CaptchaHandler,
	contentOps *contentopshandler.Handler,
	ai *aihandler.Handler,
	social *socialhandler.Handler,
	video *videohandler.Handler,
	videoAdmin *videohandler.AdminHandler,
	comment *commenthandler.Handler,
	chat *chathandler.Handler,
	notification *notificationhandler.Handler,
	dynamic *dynamichandler.Handler,
	rbac *rbachandler.Handler,
) *Router {
	return &Router{
		cfgProvider:   cfgProvider,
		mw:            mw,
		validator:     validator,
		identity:      identity,
		identityAdmin: identityAdmin,
		captcha:       captcha,
		contentOps:    contentOps,
		ai:            ai,
		social:        social,
		video:         video,
		videoAdmin:    videoAdmin,
		comment:       comment,
		chat:          chat,
		notification:  notification,
		dynamic:       dynamic,
		rbac:          rbac,
	}
}

// BuildEngine 构建并返回配置完毕的 gin 引擎。
// 语义与原 router.buildEngine 一致（gin.Default + 代理信任配置 + Cors + 静态目录 + 路由分组）。
func (r *Router) BuildEngine() *gin.Engine {
	cfg := r.cfgProvider.Get()
	if cfg != nil && cfg.Server != nil && cfg.Server.Mode != "" {
		gin.SetMode(cfg.Server.Mode)
	}

	engine := gin.Default()
	engine.ContextWithFallback = true
	if r.mw != nil {
		engine.Use(r.mw.InjectLogger())
	}
	if r.validator != nil {
		engine.Use(r.validator.Middleware())
	}
	// 信任代理转发的客户端 IP 头（Nginx / Vite / CDN）
	// 默认 RemoteIPHeaders 已含 X-Forwarded-For / X-Real-IP，这里补充 CDN 常用头。
	engine.RemoteIPHeaders = []string{"CF-Connecting-IP", "True-Client-IP", "X-Forwarded-For", "X-Real-IP"}
	// 开发与常见内网反代场景：信任回环与私有网段代理；生产若直暴露公网可再收紧。
	_ = engine.SetTrustedProxies([]string{
		"127.0.0.0/8",
		"::1/128",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	})
	engine.Use(middlewares.Cors(r.cfgProvider))
	engine.Static(staticUploadsRoute, staticUploadsDir)

	v1Group := engine.Group("/v1")
	commonGroup := v1Group.Group("/common")
	r.registerCommonRoutes(commonGroup)

	adminGroup := v1Group.Group("/admin")
	r.registerAdminRoutes(adminGroup)

	return engine
}
