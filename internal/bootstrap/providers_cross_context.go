package bootstrap

import (
	aiapp "f_ccg_video_back_ddd/internal/application/ai"
	chatapp "f_ccg_video_back_ddd/internal/application/chat"
	commentapp "f_ccg_video_back_ddd/internal/application/comment"
	contentopsapp "f_ccg_video_back_ddd/internal/application/contentops"
	dynamicapp "f_ccg_video_back_ddd/internal/application/dynamic"
	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	mediaapp "f_ccg_video_back_ddd/internal/application/media"
	platformapp "f_ccg_video_back_ddd/internal/application/platform"
	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"
	searchapp "f_ccg_video_back_ddd/internal/application/search"
	socialapp "f_ccg_video_back_ddd/internal/application/social"
	videoapp "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/bootstrap/adapters"
	ratelimitinfra "f_ccg_video_back_ddd/internal/infrastructure/cache/ratelimit"
	ipregioninfra "f_ccg_video_back_ddd/internal/infrastructure/ipregion"
	commentrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/comment"
	videorepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"
	schedulerinfra "f_ccg_video_back_ddd/internal/infrastructure/scheduler"
	"f_ccg_video_back_ddd/pkg/utils/ipUtils"

	"github.com/google/wire"
)

// CrossContextProviderSet 在组合根集中装配各限界上下文之间的显式端口。
//
// 方法集完全一致时直接 bind；DTO 或返回类型不同时使用 adapters 中的薄转换层。
var CrossContextProviderSet = wire.NewSet(
	ConfigProviderSet,
	InfrastructureProviderSet,
	SharedProviderSet,
	IdentityProviderSet,
	NotificationProviderSet,
	RbacProviderSet,
	ContentOpsProviderSet,
	CommentProviderSet,
	DynamicProviderSet,
	SocialProviderSet,
	ChatProviderSet,
	AIProviderSet,
	SearchProviderSet,
	SearchIndexProviderSet,
	MediaProviderSet,
	VideoProviderSet,
	PlatformProviderSet,
	WebSocketProviderSet,
	wire.Bind(new(schedulerinfra.UploadCleanupJob), new(*videoapp.UploadCleanupService)),

	wire.Bind(new(identityapp.RateLimiter), new(*ratelimitinfra.Limiter)),
	adapters.NewBadgeInvalidator,
	wire.Bind(new(chatapp.NotificationBadgeInvalidator), new(*adapters.BadgeInvalidator)),
	adapters.NewNotifier,
	wire.Bind(new(commentapp.Notifier), new(*adapters.Notifier)),
	wire.Bind(new(dynamicapp.DynamicLikeNotifier), new(*adapters.Notifier)),
	wire.Bind(new(videoapp.VideoNotifier), new(*adapters.Notifier)),

	adapters.NewIdentityVideoQuery,
	wire.Bind(new(identityapp.UserVideoQuery), new(*adapters.IdentityVideoQuery)),
	adapters.NewIdentityPublishedVideoStats,
	wire.Bind(new(identityapp.PublishedVideoStatsQuery), new(*adapters.IdentityPublishedVideoStats)),
	adapters.NewIdentityVideoTagWriter,
	wire.Bind(new(identityapp.VideoTagWriter), new(*adapters.IdentityVideoTagWriter)),
	wire.Bind(new(identityapp.VideoRemover), new(*videoapp.DeleteService)),
	wire.Bind(new(identityapp.VideoAuthorRenamer), new(*videorepo.AuthorRenameRepository)),
	adapters.NewIdentityVideoCacheInvalidator,
	wire.Bind(new(identityapp.VideoCacheInvalidator), new(*adapters.IdentityVideoCacheInvalidator)),
	adapters.NewIdentitySearchQuery,
	wire.Bind(new(identityapp.UserSearchQuery), new(*adapters.IdentitySearchQuery)),
	wire.Bind(new(identityapp.MentionSuggestQuery), new(*adapters.IdentitySearchQuery)),
	adapters.NewIdentityBannerStorage,
	wire.Bind(new(identityapp.BannerStorage), new(*adapters.IdentityBannerStorage)),
	adapters.NewIdentityUserRoleBinder,
	wire.Bind(new(identityapp.UserRoleBinder), new(*adapters.IdentityUserRoleBinder)),
	adapters.NewIdentityDefaultFolderEnsurer,
	wire.Bind(new(identityapp.DefaultFolderEnsurer), new(*adapters.IdentityDefaultFolderEnsurer)),
	wire.Bind(new(ipUtils.RegionResolver), new(*ipregioninfra.Resolver)),

	wire.Bind(new(contentopsapp.LoginExpGranter), new(*identityapp.ExpService)),
	wire.Bind(new(contentopsapp.LoginCoinGranter), new(*identityapp.CoinService)),
	wire.Bind(new(contentopsapp.OAuthStateStore), new(*identityapp.OAuthLoginService)),

	adapters.NewAIVideoVectorSearch,
	wire.Bind(new(aiapp.VideoVectorSearch), new(*adapters.AIVideoVectorSearch)),
	wire.Bind(new(rbacapp.MenuSuggestionGenerator), new(*aiapp.Service)),
	wire.Bind(new(socialapp.CreatorFansMetric), new(*identityapp.CreatorAnalyticsService)),
	wire.Bind(new(socialapp.UserSearchSyncer), new(*searchapp.UserIndexService)),
	wire.Bind(new(platformapp.UserStatusWriter), new(*identityapp.UserStatusService)),

	wire.Bind(new(videoapp.CreatorAnalytics), new(*identityapp.CreatorAnalyticsService)),
	adapters.NewUploadQuotaExemption,
	wire.Bind(new(videoapp.UploadQuotaExemption), new(*adapters.UploadQuotaExemption)),
	wire.Bind(new(videoapp.ExpAwarder), new(*identityapp.ExpService)),
	adapters.NewIdentityCoinLedger,
	wire.Bind(new(videoapp.CoinLedger), new(*adapters.IdentityCoinLedger)),
	adapters.NewVideoStorageResolver,
	wire.Bind(new(videoapp.StorageResolver), new(*adapters.VideoStorageResolver)),
	adapters.NewVideoTranscodeDispatcher,
	wire.Bind(new(videoapp.TranscodeDispatcher), new(*adapters.VideoTranscodeDispatcher)),
	adapters.NewVideoTranscodeProgressReader,
	wire.Bind(new(videoapp.TranscodeProgressReader), new(*adapters.VideoTranscodeProgressReader)),
	adapters.NewVideoSearcher,
	wire.Bind(new(videoapp.VideoSearcher), new(*adapters.VideoSearcher)),
	wire.Bind(new(videoapp.VideoVectorIndexer), new(*searchapp.VideoVectorService)),
	adapters.NewVideoVectorRebuilder,
	wire.Bind(new(videoapp.VectorIndexRebuilder), new(*adapters.VideoVectorRebuilder)),
	wire.Bind(new(videoapp.DynamicReadMarker), new(*dynamicapp.ReadService)),
	wire.Bind(new(videoapp.CommentPurger), new(*commentrepo.PurgeRepository)),
	wire.Bind(new(videoapp.TranscodeCanceller), new(*mediaapp.CancelService)),

	wire.Bind(new(commentapp.VideoCommentCounter), new(*videoapp.StatsService)),
	wire.Bind(new(commentapp.VideoHotScoreRecorder), new(*videoapp.StatsService)),

	wire.Bind(new(dynamicapp.VideoPinWriter), new(*videorepo.PinRepository)),
	wire.Bind(new(dynamicapp.DynamicCommentPurger), new(*commentrepo.PurgeRepository)),
)
