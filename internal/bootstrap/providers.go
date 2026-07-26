package bootstrap

import (
	"f_ccg_video_back_ddd/internal/infrastructure/auth"
	"f_ccg_video_back_ddd/internal/infrastructure/cache"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/ipregion"
	"f_ccg_video_back_ddd/internal/infrastructure/logger"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/mongodb"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/mysql"
	"f_ccg_video_back_ddd/internal/infrastructure/search"
	"f_ccg_video_back_ddd/internal/interfaces/cli"
	"f_ccg_video_back_ddd/internal/interfaces/http/router"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/google/wire"
)

// ConfigProviderSet 配置装载与窄化 provider。
//
// 窄化 provider 把根配置拆成各基础组件真正需要的最小配置结构体，
// 需要读取热更新配置的组件则直接注入 *config.Provider。
var ConfigProviderSet = wire.NewSet(
	cli.ProvideConfigPath,
	config.Load,
	config.ProvideServerConf,
	config.ProvideLoggerConf,
	config.ProvideMysqlConf,
	config.ProvideRedisConf,
	config.ProvideMongoConf,
	config.ProvideESConf,
	config.ProvideKafkaConf,
	config.ProvideRiverConf,
	config.ProvideTranscodeConf,
	config.ProvideJwtConf,
)

// InfrastructureProviderSet 基础设施客户端 provider（按技术分组）。
var InfrastructureProviderSet = wire.NewSet(
	logger.Provide,
	logger.ProvideZap,
	mysql.Provide,
	mysql.ProvideGorm,
	cache.Provide,
	cache.ProvideClient,
	mongodb.Provide,
	mongodb.ProvideDatabase,
	search.Provide,
	search.ProvideV7,
	search.ProvideV8,
	ipregion.Provide,
	ipregion.ProvideXDB,
	ipregion.NewResolver,
	auth.NewEnforcer,
	ctxutil.NewDetachedTaskGroup,
	NewInfrastructure,
)

// InterfacesProviderSet 接口层 provider（HTTP 路由装配、处理器）。
var InterfacesProviderSet = wire.NewSet(
	router.New,
)

// AppProviderSet 应用组合根的完整 provider 集合。
var AppProviderSet = wire.NewSet(
	ConfigProviderSet,
	InfrastructureProviderSet,
	InterfacesProviderSet,
	NewApp,
)
