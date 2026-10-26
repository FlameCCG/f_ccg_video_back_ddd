package bootstrap

import (
	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	notificationapp "f_ccg_video_back_ddd/internal/application/notification"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	notificationdomain "f_ccg_video_back_ddd/internal/domain/notification"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	identitycache "f_ccg_video_back_ddd/internal/infrastructure/cache/identity"
	notificationcache "f_ccg_video_back_ddd/internal/infrastructure/cache/notification"
	"f_ccg_video_back_ddd/internal/infrastructure/cache/ratelimit"
	"f_ccg_video_back_ddd/internal/infrastructure/captcha"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	identityrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"
	notificationrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/notification"

	"github.com/google/wire"
)

// SharedProviderSet 跨上下文共用的基础能力（事务编排、限流）。
//
// 同一个 TxManager 的实现通过 context 传递数据库事务。跨上下文端口若最终使用同库且保留该 context，
// 可以参加本地事务；仅仅绑定到了同一个接口，并不能使外部系统获得事务语义。
var SharedProviderSet = wire.NewSet(
	gormx.NewTxManager,
	wire.Bind(new(tx.Manager), new(*gormx.TxManager)),
	ratelimit.New,
)

// IdentityProviderSet 身份上下文：仓储与缓存实现 + 应用服务 + 领域接口绑定。
//
// 对照每组 NewX 与 wire.Bind 阅读：前者产出具体类型，后者声明它能交给哪一个业务接口。
// 同一具体对象可以满足多个窄接口，避免让每个使用者依赖完整仓储能力。
var IdentityProviderSet = wire.NewSet(
	// 持久化实现
	identityrepo.NewUserRepository,
	wire.Bind(new(identitydomain.UserRepository), new(*identityrepo.UserRepository)),
	wire.Bind(new(identitydomain.UserAccountRepository), new(*identityrepo.UserRepository)),
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
	identityrepo.NewUserBannerQuery,
	wire.Bind(new(identityapp.UserBannerCatalog), new(*identityrepo.UserBannerQuery)),
	identityrepo.NewUserConfRepository,
	wire.Bind(new(identitydomain.UserConfRepository), new(*identityrepo.UserConfRepository)),
	wire.Bind(new(identityapp.UserConfQuery), new(*identityrepo.UserConfRepository)),
	identityrepo.NewUserRelationQuery,
	wire.Bind(new(identityapp.UserRelationQuery), new(*identityrepo.UserRelationQuery)),
	identityrepo.NewUserRoleQuery,
	wire.Bind(new(identityapp.UserRoleQuery), new(*identityrepo.UserRoleQuery)),

	// 缓存实现
	identitycache.NewUserStatusCache,
	wire.Bind(new(identitydomain.UserStatusCache), new(*identitycache.UserStatusCache)),
	identitycache.NewCreatorMetricStore,
	wire.Bind(new(identityapp.CreatorMetricStore), new(*identitycache.CreatorMetricStore)),

	// 验证码适配器
	captcha.NewEmailGenerator,
	wire.Bind(new(identityapp.EmailCaptchaGenerator), new(*captcha.EmailGenerator)),
	captcha.NewClickCaptcha,
	wire.Bind(new(identityapp.ClickCaptchaPort), new(*captcha.ClickCaptcha)),
	captcha.NewSlideCaptcha,
	wire.Bind(new(identityapp.SlideCaptchaPort), new(*captcha.SlideCaptcha)),
	captcha.NewGraphicStore,
	captcha.NewEmailCaptchaStore,
	wire.Bind(new(identityapp.EmailCaptchaStore), new(*captcha.EmailCaptchaStore)),

	// 应用服务
	identityapp.ProviderSet,
	wire.Bind(new(identityapp.UserStatusReader), new(*identityapp.UserStatusService)),
)

// NotificationProviderSet 通知上下文：仓储与角标缓存 + 应用服务 + 跨上下文端口绑定。
var NotificationProviderSet = wire.NewSet(
	notificationrepo.NewRepository,
	wire.Bind(new(notificationdomain.Repository), new(*notificationrepo.Repository)),
	notificationcache.NewCountCache,
	wire.Bind(new(notificationapp.CountCache), new(*notificationcache.CountCache)),
	notificationrepo.NewActorQuery,
	wire.Bind(new(notificationapp.ActorProfileProvider), new(*notificationrepo.ActorQuery)),
	notificationapp.ProviderSet,
	// identity 的封禁通知端口由通知应用服务实现（跨上下文协调在应用层完成）
	wire.Bind(new(identityapp.BanNotifier), new(*notificationapp.Service)),
)
