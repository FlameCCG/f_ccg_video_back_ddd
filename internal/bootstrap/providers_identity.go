package bootstrap

import (
	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	notificationapp "f_ccg_video_back_ddd/internal/application/notification"
	provisioningapp "f_ccg_video_back_ddd/internal/application/provisioning"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	notificationdomain "f_ccg_video_back_ddd/internal/domain/notification"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	identitycache "f_ccg_video_back_ddd/internal/infrastructure/cache/identity"
	notificationcache "f_ccg_video_back_ddd/internal/infrastructure/cache/notification"
	"f_ccg_video_back_ddd/internal/infrastructure/cache/ratelimit"
	"f_ccg_video_back_ddd/internal/infrastructure/captcha"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	identityrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/migration"
	notificationrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/notification"
	provisioningrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/provisioning"

	"github.com/google/wire"
)

// SharedProviderSet 跨上下文共用的基础能力（事务编排、限流）。
var SharedProviderSet = wire.NewSet(
	gormx.NewTxManager,
	wire.Bind(new(tx.Manager), new(*gormx.TxManager)),
	ratelimit.New,
)

// IdentityProviderSet 身份上下文：仓储与缓存实现 + 应用服务 + 领域接口绑定。
var IdentityProviderSet = wire.NewSet(
	// 持久化实现
	identityrepo.NewUserRepository,
	wire.Bind(new(identitydomain.UserRepository), new(*identityrepo.UserRepository)),
	// CLI 建号任务复用同一份「默认横幅 ID 注入 ctx」逻辑（见 report.md FIND-002）
	wire.Bind(new(provisioningrepo.UserContextDecorator), new(*identityrepo.UserRepository)),
	identityrepo.NewExpRecordRepository,
	wire.Bind(new(identitydomain.ExpRecordRepository), new(*identityrepo.ExpRecordRepository)),
	identityrepo.NewCoinRecordRepository,
	wire.Bind(new(identitydomain.CoinRecordRepository), new(*identityrepo.CoinRecordRepository)),
	identityrepo.NewBanRecordRepository,
	wire.Bind(new(identitydomain.BanRecordRepository), new(*identityrepo.BanRecordRepository)),
	identityrepo.NewLoginRecordRepository,
	wire.Bind(new(identitydomain.LoginRecordRepository), new(*identityrepo.LoginRecordRepository)),

	// 读侧查询实现
	identityrepo.NewAdminUserQuery,
	wire.Bind(new(identityapp.AdminUserQuery), new(*identityrepo.AdminUserQuery)),
	identityrepo.NewBanRecordQuery,
	wire.Bind(new(identityapp.BanRecordQuery), new(*identityrepo.BanRecordQuery)),
	identityrepo.NewCoinRecordQuery,
	wire.Bind(new(identityapp.CoinRecordQuery), new(*identityrepo.CoinRecordQuery)),
	identityrepo.NewExpRecordQuery,
	wire.Bind(new(identityapp.ExpRecordQuery), new(*identityrepo.ExpRecordQuery)),
	identityrepo.NewLoginRecordQuery,
	wire.Bind(new(identityapp.LoginRecordQuery), new(*identityrepo.LoginRecordQuery)),
	identityrepo.NewCreatorVideoStatsQuery,
	wire.Bind(new(identityapp.CreatorVideoStatsQuery), new(*identityrepo.CreatorVideoStatsQuery)),
	identityrepo.NewFansCounter,
	wire.Bind(new(identityapp.FansCounter), new(*identityrepo.FansCounter)),

	// 缓存实现
	identitycache.NewUserStatusCache,
	wire.Bind(new(identitydomain.UserStatusCache), new(*identitycache.UserStatusCache)),
	identitycache.NewCreatorMetricStore,
	wire.Bind(new(identityapp.CreatorMetricStore), new(*identitycache.CreatorMetricStore)),

	// 验证码适配器
	captcha.NewClickCaptcha,
	wire.Bind(new(identityapp.ClickCaptchaPort), new(*captcha.ClickCaptcha)),
	captcha.NewSlideCaptcha,
	wire.Bind(new(identityapp.SlideCaptchaPort), new(*captcha.SlideCaptcha)),
	captcha.NewGraphicStore,
	captcha.NewEmailCaptchaStore,
	wire.Bind(new(identityapp.EmailCaptchaStore), new(*captcha.EmailCaptchaStore)),

	// 应用服务
	identityapp.NewUserStatusService,
	wire.Bind(new(identityapp.UserStatusReader), new(*identityapp.UserStatusService)),
	identityapp.NewExpService,
	identityapp.NewCoinService,
	identityapp.NewBanService,
	identityapp.NewBanRecordService,
	identityapp.NewCoinRecordService,
	identityapp.NewExpRecordService,
	identityapp.NewLoginRecordService,
	identityapp.NewAdminUserService,
	identityapp.NewCreatorAnalyticsService,
	identityapp.NewCaptchaService,
)

// NotificationProviderSet 通知上下文：仓储与角标缓存 + 应用服务 + 跨上下文端口绑定。
var NotificationProviderSet = wire.NewSet(
	notificationrepo.NewRepository,
	wire.Bind(new(notificationdomain.Repository), new(*notificationrepo.Repository)),
	notificationcache.NewCountCache,
	wire.Bind(new(notificationapp.CountCache), new(*notificationcache.CountCache)),
	notificationapp.NewService,
	// identity 的封禁通知端口由通知应用服务实现（跨上下文协调在应用层完成）
	wire.Bind(new(identityapp.BanNotifier), new(*notificationapp.Service)),
)

// ProvisioningProviderSet 运维初始化任务：数据库迁移、超管创建、Banner 初始化。
var ProvisioningProviderSet = wire.NewSet(
	migration.NewMigrator,
	provisioningrepo.NewSuperUserRepository,
	wire.Bind(new(provisioningapp.SuperUserRepository), new(*provisioningrepo.SuperUserRepository)),
	provisioningrepo.NewCasbinRoleBinder,
	wire.Bind(new(provisioningapp.CasbinRoleBinder), new(*provisioningrepo.CasbinRoleBinder)),
	provisioningrepo.NewBannerRepository,
	wire.Bind(new(provisioningapp.BannerRepository), new(*provisioningrepo.BannerRepository)),
	provisioningapp.NewSuperUserService,
	provisioningapp.NewBannerService,
)
