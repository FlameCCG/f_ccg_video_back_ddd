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

// ConfigProviderSet 配置装载与窄化 provider。
//
// 窄化 provider 把根配置拆成各基础组件真正需要的最小配置结构体，
// 需要读取热更新配置的基础组件直接注入 *infraconfig.Provider；
// application 层一律经 shared.ConfigReader / shared.ConfigStore 端口注入（D-020）。
var ConfigProviderSet = wire.NewSet(
	cli.ProvideConfigPath,
	infraconfig.Load,
	wire.Bind(new(shared.ConfigReader), new(*infraconfig.Provider)),
	wire.Bind(new(shared.ConfigStore), new(*infraconfig.Provider)),
	infraconfig.ProvideServerConf,
	infraconfig.ProvideLoggerConf,
	infraconfig.ProvideMysqlConf,
	infraconfig.ProvideRedisConf,
	infraconfig.ProvideMongoConf,
	infraconfig.ProvideESConf,
	infraconfig.ProvideKafkaConf,
	infraconfig.ProvideRiverConf,
	infraconfig.ProvideTranscodeConf,
	infraconfig.ProvideJwtConf,
)

// InfrastructureProviderSet 基础设施客户端 provider（按技术分组）。
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
var InterfacesProviderSet = wire.NewSet(
	validate.New,
	middlewares.New,
	router.New,
	NewRuntimeComponents,
)

// AppProviderSet 应用组合根的完整 provider 集合。
var AppProviderSet = wire.NewSet(
	ProvisioningProviderSet,
	CrossContextProviderSet,
	IdentityHTTPProviderSet,
	NotificationHTTPProviderSet,
	InterfacesProviderSet,
	NewApp,
)
