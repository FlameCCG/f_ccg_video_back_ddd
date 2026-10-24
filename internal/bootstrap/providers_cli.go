package bootstrap

import (
	provisioningapp "f_ccg_video_back_ddd/internal/application/provisioning"
	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"
	mediadomain "f_ccg_video_back_ddd/internal/domain/media"
	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/internal/infrastructure/auth"
	"f_ccg_video_back_ddd/internal/infrastructure/logger"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	identityrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/migration"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/mysql"
	provisioningrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/provisioning"
	rbacrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/rbac"
	"f_ccg_video_back_ddd/internal/infrastructure/search"
	"f_ccg_video_back_ddd/internal/infrastructure/storage"
	"f_ccg_video_back_ddd/internal/interfaces/cli"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"f_ccg_video_back_ddd/pkg/validate"

	"github.com/google/wire"
)

// CLIDatabaseProviderSet 是 -db/-vp 的最小初始化图：配置 → 日志 → MySQL。
var CLIDatabaseProviderSet = wire.NewSet(
	ConfigProviderSet,
	loggerProviderSet,
	mysql.Provide,
	mysql.ProvideGorm,
)

// loggerProviderSet 让所有 CLI 图显式获得同一实例的 *zap.Logger。
var loggerProviderSet = wire.NewSet(
	logger.Provide,
	logger.ProvideZap,
)

// MigrateDBTaskProviderSet 组装 -db 用例。
var MigrateDBTaskProviderSet = wire.NewSet(
	CLIDatabaseProviderSet,
	migration.NewMigrator,
	wire.Bind(new(cli.DatabaseMigrator), new(*migration.Migrator)),
	cli.NewMigrateDBExecutor,
)

// VideoPartitionsTaskProviderSet 组装 -vp 用例。
var VideoPartitionsTaskProviderSet = wire.NewSet(
	CLIDatabaseProviderSet,
	migration.NewMigrator,
	wire.Bind(new(cli.VideoPartitionSeeder), new(*migration.Migrator)),
	cli.NewVideoPartitionsExecutor,
)

// BannerTaskProviderSet 组装 -br；只在数据库图上增加存储解析器。
var BannerTaskProviderSet = wire.NewSet(
	CLIDatabaseProviderSet,
	ctxutil.NewDetachedTaskGroup,
	storage.NewMinIOClientPool,
	storage.NewResolver,
	wire.Bind(new(mediadomain.StorageResolver), new(*storage.Resolver)),
	provisioningrepo.NewBannerRepository,
	wire.Bind(new(provisioningapp.BannerRepository), new(*provisioningrepo.BannerRepository)),
	NewBannerStorageResolverAdapter,
	wire.Bind(new(cli.BannerStorageResolver), new(*BannerStorageResolverAdapter)),
	cli.NewBannerExecutor,
)

// SuperUserTaskProviderSet 组装 -su；Casbin 只出现在该早期数据库任务的图中。
var SuperUserTaskProviderSet = wire.NewSet(
	CLIDatabaseProviderSet,
	gormx.NewTxManager,
	wire.Bind(new(tx.Manager), new(*gormx.TxManager)),
	auth.NewEnforcer,
	identityrepo.NewUserRepository,
	wire.Bind(new(provisioningrepo.UserContextDecorator), new(*identityrepo.UserRepository)),
	provisioningrepo.NewSuperUserRepository,
	wire.Bind(new(provisioningapp.SuperUserRepository), new(*provisioningrepo.SuperUserRepository)),
	provisioningrepo.NewCasbinRoleBinder,
	wire.Bind(new(provisioningapp.CasbinRoleBinder), new(*provisioningrepo.CasbinRoleBinder)),
	provisioningapp.NewSuperUserService,
	wire.Bind(new(cli.SuperUserProvisioner), new(*provisioningapp.SuperUserService)),
	cli.NewSuperUserExecutor,
)

// ESIndexTaskProviderSet 组装 -es；NewESCLIExecutor 的 *mysql.DB 参数保证 MySQL 先初始化。
var ESIndexTaskProviderSet = wire.NewSet(
	CLIDatabaseProviderSet,
	search.Provide,
	search.ProvideV7,
	search.NewIndexManager,
	NewESCLIExecutor,
)

// CLIFullInfrastructureProviderSet 对应原 InitCore 在 -rs/-rbac 前完成的全部初始化节点。
var CLIFullInfrastructureProviderSet = wire.NewSet(
	ConfigProviderSet,
	InfrastructureProviderSet,
)

// RBACInitTaskProviderSet 只组装默认 RBAC 初始化用例，同时强制完整前置基础设施就绪。
var RBACInitTaskProviderSet = wire.NewSet(
	CLIFullInfrastructureProviderSet,
	rbacrepo.NewCasbinEnforcer,
	wire.Bind(new(rbacapp.Enforcer), new(*rbacrepo.CasbinEnforcer)),
	rbacrepo.NewRoleRepository,
	wire.Bind(new(rbacdomain.RoleRepository), new(*rbacrepo.RoleRepository)),
	rbacrepo.NewMenuRepository,
	wire.Bind(new(rbacdomain.MenuRepository), new(*rbacrepo.MenuRepository)),
	rbacrepo.NewRoleMenuRepository,
	wire.Bind(new(rbacdomain.RoleMenuRepository), new(*rbacrepo.RoleMenuRepository)),
	rbacapp.NewInitService,
	NewRBACInitCLIExecutor,
)

// ResourceSyncTaskProviderSet 使用真实 Router 构建路由清单；只初始化原 -rs 前置基础设施，
// 不构造完整 App，也不提前连接 Kafka/启动后台组件。
var ResourceSyncTaskProviderSet = wire.NewSet(
	CLIFullInfrastructureProviderSet,
	rbacrepo.NewApiResourceRepository,
	wire.Bind(new(rbacdomain.ApiResourceRepository), new(*rbacrepo.ApiResourceRepository)),
	rbacrepo.NewOpenAPIMetaLoader,
	wire.Bind(new(rbacapp.ResourceMetaLoader), new(*rbacrepo.OpenAPIMetaLoader)),
	validate.New,
	NewCLIRouter,
	NewRouterRouteLister,
	wire.Bind(new(rbacapp.RouteLister), new(*RouterRouteLister)),
	rbacapp.NewResourceSyncService,
	NewResourceSyncCLIExecutor,
)
