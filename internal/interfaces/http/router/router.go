// Package router 装配 HTTP 路由、中间件链和各上下文处理器。
//
// router.go 构建 Gin Engine，register.go 与 *_router.go 描述实际 method/path 和入口所需的权限链。
// 注册是协议装配，不做业务处理；Handler 再将请求翻译为应用用例调用。
// RouteSpecs 可由 CLI 读取用于 API 资源同步，复用注册逻辑以避免维护第二份路由真相。
// 测试将 engine.Routes() 与 testdata/routes_golden.txt 对比；新增路由需同时评估权限和契约变化。
// /uploads 是静态资源入口，其 GET/HEAD 行为也属于路由契约。
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
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/validate"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

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
	cfgProvider *infraconfig.Provider
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
	cfgProvider *infraconfig.Provider,
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
// 使用显式请求上下文、日志和恢复链，再装配代理信任、Cors、静态目录与业务路由。
func (r *Router) BuildEngine() *gin.Engine {
	cfg := r.cfgProvider.Get()
	if cfg != nil && cfg.Server != nil && cfg.Server.Mode != "" {
		gin.SetMode(cfg.Server.Mode)
	}

	engine := gin.New()
	engine.Use(middlewares.RequestContext(), r.mw.RequestLogger(), r.mw.Recovery())

	engine.ContextWithFallback = true
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
