package bootstrap

import (
	shared "f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/infrastructure/auth"
	"f_ccg_video_back_ddd/internal/infrastructure/cache"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/ipregion"
	"f_ccg_video_back_ddd/internal/infrastructure/logger"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/mongodb"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/mysql"
	"f_ccg_video_back_ddd/internal/infrastructure/search"
	"f_ccg_video_back_ddd/internal/interfaces/cli"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/internal/interfaces/http/router"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"f_ccg_video_back_ddd/pkg/validate"

	"github.com/google/wire"
)

// configValueProviderSet 是配置路径与窄化 provider 的公共声明。
//
// 窄化 provider 把根配置拆成各基础组件真正需要的最小配置结构体，
// 需要读取热更新配置的基础组件直接注入 *infraconfig.Provider；
// application 层一律经 shared.ConfigReader / shared.ConfigStore 端口注入（D-020）。
var configValueProviderSet = wire.NewSet(
	cli.ProvideConfigPath,
	infraconfig.ProvideLoggerConf,
	infraconfig.ProvideMysqlConf,
	infraconfig.ProvideRedisConf,
	infraconfig.ProvideMongoConf,
	infraconfig.ProvideESConf,
	infraconfig.ProvideKafkaConf,
	infraconfig.ProvideRiverConf,
)

// ConfigProviderSet 为 HTTP 服务加载配置并开启热更新。
var ConfigProviderSet = wire.NewSet(
	configValueProviderSet,
	infraconfig.Load,
	wire.Bind(new(shared.ConfigReader), new(*infraconfig.Provider)),
	wire.Bind(new(shared.ConfigStore), new(*infraconfig.Provider)),
)

// InfrastructureProviderSet 基础设施客户端 provider（按技术分组）。
//
// NewSet 汇总候选构造器，不是启动脚本，也不保证写在集合里的每个构造器都会执行。
// Wire 从注入器输出反向取可达依赖；具体构建和错误清理顺序可以在 wire_gen.go 中核对。
var InfrastructureProviderSet = wire.NewSet(
	logger.ProviderSet,
	mysql.ProviderSet,
	cache.ProviderSet,
	mongodb.ProviderSet,
	search.ProviderSet,
	ipregion.ProviderSet,
	auth.ProviderSet,
	ctxutil.NewDetachedTaskGroup,
	NewInfrastructure,
)

// InterfacesProviderSet 接口层 provider（HTTP 路由装配、处理器）。
//
// 依赖图的边表示“构造时需要哪个类型”，不表示“收到请求后调用哪个业务”。
// Router 与 RuntimeComponents 都必须被 App 持有，HTTP 和后台能力才会一起进入装配图。
var InterfacesProviderSet = wire.NewSet(
	validate.New,
	middlewares.New,
	router.New,
	NewRuntimeComponents,
)

// AppProviderSet 应用组合根的完整 provider 集合。
//
// 配置与基础设施经 CrossContextProviderSet 引入，NewApp 是本注入器最终返回对象的构造器。
// 阅读重复嵌套的集合时沿类型追踪，不要把集合层级误认为另一套运行期业务分层。
var AppProviderSet = wire.NewSet(
	CrossContextProviderSet,
	IdentityHTTPProviderSet,
	NotificationHTTPProviderSet,
	InterfacesProviderSet,
	NewApp,
)
