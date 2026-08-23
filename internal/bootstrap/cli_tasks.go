package bootstrap

import (
	provisioningapp "f_ccg_video_back_ddd/internal/application/provisioning"
	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"
	mediadomain "f_ccg_video_back_ddd/internal/domain/media"
	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/mysql"
	"f_ccg_video_back_ddd/internal/infrastructure/search"
	"f_ccg_video_back_ddd/internal/interfaces/cli"
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
	"f_ccg_video_back_ddd/internal/interfaces/http/router"
	"f_ccg_video_back_ddd/pkg/validate"

	"github.com/casbin/casbin/v2"
	"go.uber.org/zap"
)

// BannerStorageResolverAdapter 把 media 的完整存储解析端口窄化为 Banner CLI 所需的 SaveFile 端口。
type BannerStorageResolverAdapter struct {
	resolver mediadomain.StorageResolver
}

var _ cli.BannerStorageResolver = (*BannerStorageResolverAdapter)(nil)

// NewBannerStorageResolverAdapter 构造 Banner 存储解析适配器。
func NewBannerStorageResolverAdapter(
	resolver mediadomain.StorageResolver,
) *BannerStorageResolverAdapter {
	return &BannerStorageResolverAdapter{resolver: resolver}
}

// Resolve 解析当前站点启用的真实存储驱动。
func (a *BannerStorageResolverAdapter) Resolve() (provisioningapp.StorageDriver, error) {
	driver, err := a.resolver.Resolve()
	if err != nil {
		return nil, err
	}

	return driver, nil
}

// NewCLIRouter 构造仅用于路由枚举的真实 Router。
//
// 路由注册只需要非 nil handler 接收者；请求处理用例不会在 -rs 进程中执行，
// 因而使用空 handler 壳可避免错误初始化 Kafka、对象存储等原 InitCore 在 -rs 之后才启动的组件。
func NewCLIRouter(
	cfgProvider *infraconfig.Provider,
	enforcer *casbin.SyncedEnforcer,
	logger *zap.Logger,
	translator *validate.Translator,
) *router.Router {
	mw := middlewares.New(cfgProvider, enforcer, nil, nil, nil, nil, nil, logger)

	return router.New(
		cfgProvider,
		mw,
		translator,
		&identityhandler.Handler{},
		&identityhandler.AdminHandler{},
		&identityhandler.CaptchaHandler{},
		&contentopshandler.Handler{},
		&aihandler.Handler{},
		&socialhandler.Handler{},
		&videohandler.Handler{},
		&videohandler.AdminHandler{},
		&commenthandler.Handler{},
		&chathandler.Handler{},
		&notificationhandler.Handler{},
		&dynamichandler.Handler{},
		&rbachandler.Handler{},
	)
}

// RouterRouteLister 把真实 HTTP Router 的 Gin 路由清单适配到 RBAC 应用端口。
type RouterRouteLister struct {
	router *router.Router
}

var _ rbacapp.RouteLister = (*RouterRouteLister)(nil)

// NewRouterRouteLister 构造真实路由清单适配器。
func NewRouterRouteLister(httpRouter *router.Router) *RouterRouteLister {
	return &RouterRouteLister{router: httpRouter}
}

// Routes 构建与服务启动相同的 Router，并把 Gin 路由转换为领域值。
func (l *RouterRouteLister) Routes() []rbacdomain.RouteInfo {
	registered := l.router.BuildEngine().Routes()
	routes := make([]rbacdomain.RouteInfo, 0, len(registered))

	for _, route := range registered {
		routes = append(routes, rbacdomain.RouteInfo{
			Method: route.Method,
			Path:   route.Path,
		})
	}

	return routes
}

// NewESCLIExecutor 强制 -es 在 ES 前先完成 MySQL 初始化，与原 InitCore 顺序一致。
func NewESCLIExecutor(
	_ *mysql.DB,
	indexes *search.IndexManager,
	logger *zap.Logger,
) *cli.Executor {
	return cli.NewESIndexExecutor(indexes, logger)
}

// NewResourceSyncCLIExecutor 强制 -rs 在执行用例前完成原 InitCore 的全部前置基础设施初始化。
func NewResourceSyncCLIExecutor(
	_ *Infrastructure,
	service *rbacapp.ResourceSyncService,
	logger *zap.Logger,
) *cli.Executor {
	return cli.NewResourceSyncExecutor(service, logger)
}

// NewRBACInitCLIExecutor 强制 -rbac 在执行用例前完成原 InitCore 的全部前置基础设施初始化。
func NewRBACInitCLIExecutor(
	_ *Infrastructure,
	service *rbacapp.InitService,
	logger *zap.Logger,
) *cli.Executor {
	return cli.NewRBACInitExecutor(service, logger)
}
