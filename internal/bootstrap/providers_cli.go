package bootstrap

import (
	contentopsapp "f_ccg_video_back_ddd/internal/application/contentops"
	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	mediaapplication "f_ccg_video_back_ddd/internal/application/media"
	provisioningapp "f_ccg_video_back_ddd/internal/application/provisioning"
	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"
	videoapp "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/bootstrap/adapters"
	contentopsdomain "f_ccg_video_back_ddd/internal/domain/contentops"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	videodomain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/auth"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/logger"
	contentopsrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/contentops"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	identityrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/migration"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/mysql"
	provisioningrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/provisioning"
	rbacrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/rbac"
	videorepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"
	"f_ccg_video_back_ddd/internal/infrastructure/search"
	"f_ccg_video_back_ddd/internal/infrastructure/storage"
	"f_ccg_video_back_ddd/internal/interfaces/cli"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"f_ccg_video_back_ddd/pkg/validate"

	"github.com/google/wire"
)

// CLIConfigProviderSet 只加载配置快照，一次性任务不创建文件监听。
var CLIConfigProviderSet = wire.NewSet(configValueProviderSet, infraconfig.LoadOnce)

// CLIDatabaseProviderSet 是 -db/-vp 的最小初始化图：配置 → 日志 → MySQL。
var CLIDatabaseProviderSet = wire.NewSet(
	CLIConfigProviderSet,
	logger.ProviderSet,
	mysql.Provide,
	mysql.ProvideGorm,
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
	wire.Bind(new(mediaapplication.StorageResolver), new(*storage.Resolver)),
	gormx.NewTxManager,
	wire.Bind(new(tx.Manager), new(*gormx.TxManager)),
	contentopsrepo.NewBannerRepository,
	wire.Bind(new(contentopsdomain.DefaultBannerRepository), new(*contentopsrepo.BannerRepository)),
	contentopsapp.NewDefaultBannerService,
	provisioningrepo.NewBannerSchema,
	wire.Bind(new(provisioningapp.BannerTableMigrator), new(*provisioningrepo.BannerSchema)),
	adapters.NewDefaultBanners,
	wire.Bind(new(provisioningapp.DefaultBanners), new(*adapters.DefaultBanners)),
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
	wire.Bind(new(identitydomain.SystemAccountRepository), new(*identityrepo.UserRepository)),
	videorepo.NewFolderRepository,
	wire.Bind(new(videodomain.FolderRepository), new(*videorepo.FolderRepository)),
	videoapp.NewDefaultFolderService,
	wire.Bind(new(identityapp.DefaultFolderInitializer), new(*videoapp.DefaultFolderService)),
	identityapp.NewSystemAccountService,
	rbacrepo.NewRoleRepository,
	wire.Bind(new(rbacdomain.RoleRepository), new(*rbacrepo.RoleRepository)),
	rbacrepo.NewUserRoleRepository,
	wire.Bind(new(rbacdomain.UserRoleRepository), new(*rbacrepo.UserRoleRepository)),
	rbacapp.NewSystemRoleService,
	adapters.NewSystemAccounts,
	wire.Bind(new(provisioningapp.SystemAccounts), new(*adapters.SystemAccounts)),
	provisioningrepo.NewCasbinRoleBinder,
	wire.Bind(new(provisioningapp.CasbinRoleBinder), new(*provisioningrepo.CasbinRoleBinder)),
	provisioningapp.NewSuperUserService,
	wire.Bind(new(cli.SuperUserProvisioner), new(*provisioningapp.SuperUserService)),
	cli.NewSuperUserExecutor,
)

// ESIndexTaskProviderSet 组装 -es，只连接索引创建所需的 Elasticsearch。
var ESIndexTaskProviderSet = wire.NewSet(
	CLIConfigProviderSet,
	logger.ProviderSet,
	search.Provide,
	search.ProvideV7,
	search.NewIndexManager,
	wire.Bind(new(cli.IndexCreator), new(*search.IndexManager)),
	cli.NewESIndexExecutor,
)

// RBACInitTaskProviderSet 组装默认 RBAC 初始化用例，只连接 MySQL 并加载 Casbin。
var RBACInitTaskProviderSet = wire.NewSet(
	CLIDatabaseProviderSet,
	auth.NewEnforcer,
	rbacrepo.NewCasbinEnforcer,
	wire.Bind(new(rbacapp.Enforcer), new(*rbacrepo.CasbinEnforcer)),
	rbacrepo.NewRoleRepository,
	wire.Bind(new(rbacdomain.RoleRepository), new(*rbacrepo.RoleRepository)),
	rbacrepo.NewMenuRepository,
	wire.Bind(new(rbacdomain.MenuRepository), new(*rbacrepo.MenuRepository)),
	rbacrepo.NewRoleMenuRepository,
	wire.Bind(new(rbacdomain.RoleMenuRepository), new(*rbacrepo.RoleMenuRepository)),
	rbacapp.NewInitService,
	wire.Bind(new(cli.RBACInitializer), new(*rbacapp.InitService)),
	cli.NewRBACInitExecutor,
)

// ResourceSyncTaskProviderSet 使用真实 Router 构建路由清单，只连接资源持久化所需的 MySQL。
var ResourceSyncTaskProviderSet = wire.NewSet(
	CLIDatabaseProviderSet,
	rbacrepo.NewApiResourceRepository,
	wire.Bind(new(rbacdomain.ApiResourceRepository), new(*rbacrepo.ApiResourceRepository)),
	rbacrepo.NewOpenAPIMetaLoader,
	wire.Bind(new(rbacapp.ResourceMetaLoader), new(*rbacrepo.OpenAPIMetaLoader)),
	validate.New,
	NewCLIRouter,
	NewRouterRouteLister,
	wire.Bind(new(rbacapp.RouteLister), new(*RouterRouteLister)),
	rbacapp.NewResourceSyncService,
	wire.Bind(new(cli.ResourceSynchronizer), new(*rbacapp.ResourceSyncService)),
	cli.NewResourceSyncExecutor,
)
