package bootstrap

import (
	applicationAi "f_ccg_video_back_ddd/internal/application/ai"
	applicationChat "f_ccg_video_back_ddd/internal/application/chat"
	applicationComment "f_ccg_video_back_ddd/internal/application/comment"
	applicationContentops "f_ccg_video_back_ddd/internal/application/contentops"
	applicationDynamic "f_ccg_video_back_ddd/internal/application/dynamic"
	applicationIdentity "f_ccg_video_back_ddd/internal/application/identity"
	applicationMedia "f_ccg_video_back_ddd/internal/application/media"
	applicationNotification "f_ccg_video_back_ddd/internal/application/notification"
	applicationPlatform "f_ccg_video_back_ddd/internal/application/platform"
	applicationRbac "f_ccg_video_back_ddd/internal/application/rbac"
	applicationSearch "f_ccg_video_back_ddd/internal/application/search"
	applicationSocial "f_ccg_video_back_ddd/internal/application/social"
	applicationVideo "f_ccg_video_back_ddd/internal/application/video"
	domainChat "f_ccg_video_back_ddd/internal/domain/chat"
	domainComment "f_ccg_video_back_ddd/internal/domain/comment"
	domainContentops "f_ccg_video_back_ddd/internal/domain/contentops"
	domainDynamic "f_ccg_video_back_ddd/internal/domain/dynamic"
	domainMedia "f_ccg_video_back_ddd/internal/domain/media"
	domainRbac "f_ccg_video_back_ddd/internal/domain/rbac"
	domainSocial "f_ccg_video_back_ddd/internal/domain/social"
	domainVideo "f_ccg_video_back_ddd/internal/domain/video"
	infrastructureCacheChat "f_ccg_video_back_ddd/internal/infrastructure/cache/chat"
	infrastructureCacheComment "f_ccg_video_back_ddd/internal/infrastructure/cache/comment"
	infrastructureCacheContentops "f_ccg_video_back_ddd/internal/infrastructure/cache/contentops"
	infrastructureCacheDynamic "f_ccg_video_back_ddd/internal/infrastructure/cache/dynamic"
	infrastructureCacheMedia "f_ccg_video_back_ddd/internal/infrastructure/cache/media"
	infrastructureCacheSearch "f_ccg_video_back_ddd/internal/infrastructure/cache/search"
	infrastructureCacheVideo "f_ccg_video_back_ddd/internal/infrastructure/cache/video"
	infrastructureExternalArk "f_ccg_video_back_ddd/internal/infrastructure/external/ark"
	infrastructureExternalEmail "f_ccg_video_back_ddd/internal/infrastructure/external/email"
	infrastructureExternalOauth "f_ccg_video_back_ddd/internal/infrastructure/external/oauth"
	infrastructureExternalVideo "f_ccg_video_back_ddd/internal/infrastructure/external/video"
	infrastructureMq "f_ccg_video_back_ddd/internal/infrastructure/mq"
	infrastructurePersistenceAi "f_ccg_video_back_ddd/internal/infrastructure/persistence/ai"
	infrastructurePersistenceChat "f_ccg_video_back_ddd/internal/infrastructure/persistence/chat"
	infrastructurePersistenceComment "f_ccg_video_back_ddd/internal/infrastructure/persistence/comment"
	infrastructurePersistenceContentops "f_ccg_video_back_ddd/internal/infrastructure/persistence/contentops"
	infrastructurePersistenceDynamic "f_ccg_video_back_ddd/internal/infrastructure/persistence/dynamic"
	infrastructurePersistenceMedia "f_ccg_video_back_ddd/internal/infrastructure/persistence/media"
	infrastructurePersistenceRbac "f_ccg_video_back_ddd/internal/infrastructure/persistence/rbac"
	infrastructurePersistenceSearch "f_ccg_video_back_ddd/internal/infrastructure/persistence/search"
	infrastructurePersistenceSocial "f_ccg_video_back_ddd/internal/infrastructure/persistence/social"
	infrastructurePersistenceVideo "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"
	infrastructureRiver "f_ccg_video_back_ddd/internal/infrastructure/river"
	infrastructureScheduler "f_ccg_video_back_ddd/internal/infrastructure/scheduler"
	infrastructureSearch "f_ccg_video_back_ddd/internal/infrastructure/search"
	infrastructureStorage "f_ccg_video_back_ddd/internal/infrastructure/storage"
	interfacesHttpHandlerAi "f_ccg_video_back_ddd/internal/interfaces/http/handler/ai"
	interfacesHttpHandlerChat "f_ccg_video_back_ddd/internal/interfaces/http/handler/chat"
	interfacesHttpHandlerComment "f_ccg_video_back_ddd/internal/interfaces/http/handler/comment"
	interfacesHttpHandlerContentops "f_ccg_video_back_ddd/internal/interfaces/http/handler/contentops"
	interfacesHttpHandlerDynamic "f_ccg_video_back_ddd/internal/interfaces/http/handler/dynamic"
	interfacesHttpHandlerIdentity "f_ccg_video_back_ddd/internal/interfaces/http/handler/identity"
	interfacesHttpHandlerNotification "f_ccg_video_back_ddd/internal/interfaces/http/handler/notification"
	interfacesHttpHandlerRbac "f_ccg_video_back_ddd/internal/interfaces/http/handler/rbac"
	interfacesHttpHandlerSocial "f_ccg_video_back_ddd/internal/interfaces/http/handler/social"
	interfacesHttpHandlerVideo "f_ccg_video_back_ddd/internal/interfaces/http/handler/video"
	interfacesWsChat "f_ccg_video_back_ddd/internal/interfaces/ws/chat"
	interfacesWsDanmu "f_ccg_video_back_ddd/internal/interfaces/ws/danmu"
	interfacesWsOrigin "f_ccg_video_back_ddd/internal/interfaces/ws/wsorigin"

	"github.com/google/wire"
)

// RbacProviderSet 装配角色权限与菜单的应用服务、SQL 仓储及 Casbin 适配器。
// API 元数据从 OpenAPI 读取，实际路由资源仍由 Router 同步。
var RbacProviderSet = wire.NewSet(
	// internal/application/rbac
	applicationRbac.ProviderSet,
	// internal/infrastructure/persistence/rbac
	infrastructurePersistenceRbac.NewApiResourceRepository,
	infrastructurePersistenceRbac.NewCasbinEnforcer,
	infrastructurePersistenceRbac.NewGlobPathMatcher,
	infrastructurePersistenceRbac.NewMenuRepository,
	infrastructurePersistenceRbac.NewOpenAPIMetaLoader,
	infrastructurePersistenceRbac.NewPermissionEvaluator,
	infrastructurePersistenceRbac.NewRoleMenuRepository,
	infrastructurePersistenceRbac.NewRoleRepository,
	infrastructurePersistenceRbac.NewUserRoleRepository,
	wire.Bind(new(domainRbac.ApiResourceRepository), new(*infrastructurePersistenceRbac.ApiResourceRepository)),
	wire.Bind(new(applicationRbac.Enforcer), new(*infrastructurePersistenceRbac.CasbinEnforcer)),
	wire.Bind(new(domainRbac.MenuRepository), new(*infrastructurePersistenceRbac.MenuRepository)),
	wire.Bind(new(applicationRbac.ResourceMetaLoader), new(*infrastructurePersistenceRbac.OpenAPIMetaLoader)),
	wire.Bind(new(domainRbac.RoleMenuRepository), new(*infrastructurePersistenceRbac.RoleMenuRepository)),
	wire.Bind(new(domainRbac.RoleRepository), new(*infrastructurePersistenceRbac.RoleRepository)),
	wire.Bind(new(domainRbac.UserRoleRepository), new(*infrastructurePersistenceRbac.UserRoleRepository)),
	// internal/interfaces/http/handler/rbac
	interfacesHttpHandlerRbac.NewHandler,
)

// ContentOpsProviderSet 装配运营用例、轮播图仓储、站点统计缓存与配置写回。
// 同一 ConfigFileWriter 同时满足运营和身份用例的窄接口。
var ContentOpsProviderSet = wire.NewSet(
	// internal/application/contentops
	applicationContentops.ProviderSet,
	// internal/infrastructure/persistence/contentops
	infrastructurePersistenceContentops.NewAdminStatsQuery,
	infrastructurePersistenceContentops.NewBannerRepository,
	infrastructurePersistenceContentops.NewConfigFileWriter,
	infrastructurePersistenceContentops.NewVideoPartitionQuery,
	wire.Bind(new(domainContentops.BannerRepository), new(*infrastructurePersistenceContentops.BannerRepository)),
	wire.Bind(new(applicationContentops.ConfigFileWriter), new(*infrastructurePersistenceContentops.ConfigFileWriter)),
	wire.Bind(new(applicationIdentity.ConfigFileWriter), new(*infrastructurePersistenceContentops.ConfigFileWriter)),
	wire.Bind(new(applicationContentops.VideoPartitionQuery), new(*infrastructurePersistenceContentops.VideoPartitionQuery)),
	wire.Bind(new(applicationContentops.AdminStatsQuery), new(*infrastructurePersistenceContentops.AdminStatsQuery)),
	// internal/infrastructure/cache/contentops
	infrastructureCacheContentops.NewBannerCache,
	infrastructureCacheContentops.NewSiteStatStore,
	wire.Bind(new(applicationContentops.BannerCache), new(*infrastructureCacheContentops.BannerCache)),
	wire.Bind(new(applicationContentops.SiteStatStore), new(*infrastructureCacheContentops.SiteStatStore)),
	// internal/interfaces/http/handler/contentops
	interfacesHttpHandlerContentops.NewHandler,
)

// CommentProviderSet 装配评论仓储、读侧投影、点赞缓存与 HTTP 入口。
// 通知和视频计数等跨上下文能力在 CrossContextProviderSet 中接入。
var CommentProviderSet = wire.NewSet(
	// internal/application/comment
	applicationComment.ProviderSet,
	// internal/infrastructure/persistence/comment
	infrastructurePersistenceComment.NewLikeRepository,
	infrastructurePersistenceComment.NewQuery,
	infrastructurePersistenceComment.NewRelatedQuery,
	infrastructurePersistenceComment.NewRepository,
	infrastructurePersistenceComment.NewPurgeRepository,
	wire.Bind(new(domainComment.Repository), new(*infrastructurePersistenceComment.Repository)),
	wire.Bind(new(domainComment.LikeRepository), new(*infrastructurePersistenceComment.LikeRepository)),
	wire.Bind(new(applicationComment.Query), new(*infrastructurePersistenceComment.Query)),
	wire.Bind(new(applicationComment.RelatedQuery), new(*infrastructurePersistenceComment.RelatedQuery)),
	// internal/infrastructure/cache/comment
	infrastructureCacheComment.NewLikeCache,
	wire.Bind(new(applicationComment.LikeCache), new(*infrastructureCacheComment.LikeCache)),
	// internal/interfaces/http/handler/comment
	interfacesHttpHandlerComment.NewHandler,
)

// DynamicProviderSet 装配动态写侧、关注流/活动查询和点赞/已读仓储。
// 视频置顶与评论清理通过跨上下文端口绑定，领域不导入对方模型。
var DynamicProviderSet = wire.NewSet(
	// internal/application/dynamic
	applicationDynamic.ProviderSet,
	// internal/infrastructure/persistence/dynamic
	infrastructurePersistenceDynamic.NewActivityQuery,
	infrastructurePersistenceDynamic.NewAdminDynamicQuery,
	infrastructurePersistenceDynamic.NewDynamicRepository,
	infrastructurePersistenceDynamic.NewFeedQuery,
	infrastructurePersistenceDynamic.NewFollowRelationQuery,
	infrastructurePersistenceDynamic.NewLikeRepository,
	infrastructurePersistenceDynamic.NewReadRepository,
	infrastructurePersistenceDynamic.NewUserProfileQuery,
	infrastructurePersistenceDynamic.NewVideoOwnershipQuery,
	wire.Bind(new(applicationDynamic.ActivityQuery), new(*infrastructurePersistenceDynamic.ActivityQuery)),
	wire.Bind(new(domainDynamic.DynamicRepository), new(*infrastructurePersistenceDynamic.DynamicRepository)),
	wire.Bind(new(domainDynamic.LikeRepository), new(*infrastructurePersistenceDynamic.LikeRepository)),
	wire.Bind(new(domainDynamic.ReadRepository), new(*infrastructurePersistenceDynamic.ReadRepository)),
	wire.Bind(new(applicationDynamic.FeedQuery), new(*infrastructurePersistenceDynamic.FeedQuery)),
	wire.Bind(new(applicationDynamic.AdminDynamicQuery), new(*infrastructurePersistenceDynamic.AdminDynamicQuery)),
	wire.Bind(new(applicationDynamic.UserProfileQuery), new(*infrastructurePersistenceDynamic.UserProfileQuery)),
	wire.Bind(new(applicationDynamic.FollowRelationQuery), new(*infrastructurePersistenceDynamic.FollowRelationQuery)),
	wire.Bind(new(applicationDynamic.VideoOwnershipQuery), new(*infrastructurePersistenceDynamic.VideoOwnershipQuery)),
	// internal/infrastructure/cache/dynamic
	infrastructureCacheDynamic.NewUnreadFollowCountCache,
	wire.Bind(new(applicationDynamic.UnreadFollowCountCache), new(*infrastructureCacheDynamic.UnreadFollowCountCache)),
	// internal/interfaces/http/handler/dynamic
	interfacesHttpHandlerDynamic.NewHandler,
)

// SocialProviderSet 装配关注关系写仓储与关系/用户列表投影。
// 本集合连接应用端口与 SQL 查询实现，隐私判断仍留在业务规则中。
var SocialProviderSet = wire.NewSet(
	// internal/application/social
	applicationSocial.ProviderSet,
	// internal/infrastructure/persistence/social
	infrastructurePersistenceSocial.NewFocusRepository,
	infrastructurePersistenceSocial.NewRelationQuery,
	infrastructurePersistenceSocial.NewUserExistsQuery,
	wire.Bind(new(domainSocial.FocusRepository), new(*infrastructurePersistenceSocial.FocusRepository)),
	wire.Bind(new(applicationSocial.RelationQuery), new(*infrastructurePersistenceSocial.RelationQuery)),
	wire.Bind(new(applicationSocial.UserExistsQuery), new(*infrastructurePersistenceSocial.UserExistsQuery)),
	// internal/interfaces/http/handler/social
	interfacesHttpHandlerSocial.NewHandler,
)

// ChatProviderSet 装配私信应用服务、Mongo 仓储、Redis 事件总线与 Hub。
// 同一个 Store 满足领域 Repository、应用 Query 和通知侧未读查询端口。
var ChatProviderSet = wire.NewSet(
	// internal/application/chat
	applicationChat.ProviderSet,
	// internal/infrastructure/persistence/chat
	infrastructurePersistenceChat.NewFollowRelationQuery,
	infrastructurePersistenceChat.NewIndexInitializer,
	infrastructurePersistenceChat.NewStore,
	wire.Bind(new(applicationChat.FollowRelationQuery), new(*infrastructurePersistenceChat.FollowRelationQuery)),
	wire.Bind(new(applicationChat.Query), new(*infrastructurePersistenceChat.Store)),
	wire.Bind(new(domainChat.Repository), new(*infrastructurePersistenceChat.Store)),
	wire.Bind(new(applicationNotification.ChatUnreadCounter), new(*infrastructurePersistenceChat.Store)),
	// internal/infrastructure/cache/chat
	infrastructureCacheChat.NewEventBus,
	wire.Bind(new(interfacesWsChat.EventBus), new(*infrastructureCacheChat.EventBus)),
	// internal/interfaces/ws/chat
	interfacesWsChat.NewHub,
	// internal/interfaces/http/handler/chat
	interfacesHttpHandlerChat.NewHandler,
)

// AIProviderSet 装配AI 编排、作品候选 SQL 投影与模型/资源上游适配器。
// 聊天、图片、视频和资源代理分别绑定窄接口，不让用例自行构造客户端。
var AIProviderSet = wire.NewSet(
	// internal/application/ai
	applicationAi.ProviderSet,
	// internal/infrastructure/persistence/ai
	infrastructurePersistenceAi.NewPromptVideoQuery,
	wire.Bind(new(applicationAi.PromptVideoQuery), new(*infrastructurePersistenceAi.PromptVideoQuery)),
	// internal/infrastructure/external/ark
	infrastructureExternalArk.NewAssetProxy,
	infrastructureExternalArk.NewChatModel,
	infrastructureExternalArk.NewImageModel,
	infrastructureExternalArk.NewVideoTaskClient,
	wire.Bind(new(applicationAi.AssetFetcher), new(*infrastructureExternalArk.AssetProxy)),
	wire.Bind(new(applicationAi.TextGenerator), new(*infrastructureExternalArk.ChatModel)),
	wire.Bind(new(applicationAi.ImageGenerator), new(*infrastructureExternalArk.ImageModel)),
	wire.Bind(new(applicationAi.VideoTaskClient), new(*infrastructureExternalArk.VideoTaskClient)),
	// internal/interfaces/http/handler/ai
	interfacesHttpHandlerAi.NewHandler,
)

// SearchProviderSet 装配搜索用例、SQL 回退、源数据投影和联想/热词缓存。
// River 作为异步关键词索引链路接入，ES 查询和向量实现由 SearchIndexProviderSet 补充。
var SearchProviderSet = wire.NewSet(
	// internal/application/search
	applicationSearch.ProviderSet,
	// internal/infrastructure/persistence/search
	infrastructurePersistenceSearch.NewHotKeywordQuery,
	infrastructurePersistenceSearch.NewMentionQuery,
	infrastructurePersistenceSearch.NewSearchFallbackQuery,
	infrastructurePersistenceSearch.NewSuggestFallbackQuery,
	infrastructurePersistenceSearch.NewUserDocumentQuery,
	infrastructurePersistenceSearch.NewVectorSourceQuery,
	wire.Bind(new(applicationSearch.HotKeywordFallbackQuery), new(*infrastructurePersistenceSearch.HotKeywordQuery)),
	wire.Bind(new(applicationSearch.MentionRelationQuery), new(*infrastructurePersistenceSearch.MentionQuery)),
	wire.Bind(new(applicationSearch.SearchFallbackQuery), new(*infrastructurePersistenceSearch.SearchFallbackQuery)),
	wire.Bind(new(applicationSearch.SuggestFallbackQuery), new(*infrastructurePersistenceSearch.SuggestFallbackQuery)),
	wire.Bind(new(applicationSearch.UserSearchDocumentQuery), new(*infrastructurePersistenceSearch.UserDocumentQuery)),
	wire.Bind(new(applicationSearch.VideoVectorSourceQuery), new(*infrastructurePersistenceSearch.VectorSourceQuery)),
	wire.Bind(new(applicationSearch.VideoVectorRebuildSource), new(*infrastructurePersistenceSearch.VectorSourceQuery)),
	// internal/infrastructure/cache/search
	infrastructureCacheSearch.NewHotKeywordStore,
	infrastructureCacheSearch.NewSuggestionCache,
	wire.Bind(new(applicationSearch.HotKeywordStore), new(*infrastructureCacheSearch.HotKeywordStore)),
	wire.Bind(new(applicationSearch.SuggestionCache), new(*infrastructureCacheSearch.SuggestionCache)),
	// internal/infrastructure/river
	infrastructureRiver.NewRiver,
	infrastructureRiver.NewRiverComponent,
)

// MediaProviderSet 装配媒体用例、资源仓储、进度/取消适配器、Kafka 与存储。
// TranscodeEngine 同时满足探测、检查点、暂存、产物检查和编码窄接口。
var MediaProviderSet = wire.NewSet(
	// internal/application/media
	applicationMedia.ProviderSet,
	// internal/infrastructure/persistence/media
	infrastructurePersistenceMedia.NewTranscodeResourceRepository,
	wire.Bind(new(domainMedia.TranscodeResourceRepository), new(*infrastructurePersistenceMedia.TranscodeResourceRepository)),
	// internal/infrastructure/cache/media
	infrastructureCacheMedia.NewCancelBroadcaster,
	infrastructureCacheMedia.NewCancelSubscriber,
	infrastructureCacheMedia.NewProgressCache,
	infrastructureCacheMedia.NewVideoDetailCache,
	wire.Bind(new(domainMedia.CancelBroadcaster), new(*infrastructureCacheMedia.CancelBroadcaster)),
	wire.Bind(new(domainMedia.ProgressReader), new(*infrastructureCacheMedia.ProgressCache)),
	wire.Bind(new(domainMedia.ProgressStore), new(*infrastructureCacheMedia.ProgressCache)),
	wire.Bind(new(domainMedia.VideoDetailCacheInvalidator), new(*infrastructureCacheMedia.VideoDetailCache)),
	// internal/infrastructure/mq
	infrastructureMq.ProviderSet,
	wire.Bind(new(domainMedia.TaskPublisher), new(*infrastructureMq.MQService)),
	wire.Bind(new(applicationMedia.MediaProber), new(*infrastructureMq.TranscodeEngine)),
	wire.Bind(new(applicationMedia.CheckpointManager), new(*infrastructureMq.TranscodeEngine)),
	wire.Bind(new(applicationMedia.SourceStager), new(*infrastructureMq.TranscodeEngine)),
	wire.Bind(new(applicationMedia.ArtifactStateChecker), new(*infrastructureMq.TranscodeEngine)),
	wire.Bind(new(applicationMedia.TranscodeEncoder), new(*infrastructureMq.TranscodeEngine)),
	wire.Bind(new(infrastructureMq.CancelSubscriberPort), new(*infrastructureCacheMedia.CancelSubscriber)),
	// internal/infrastructure/storage
	infrastructureStorage.ProviderSet,
	wire.Bind(new(domainMedia.StorageResolver), new(*infrastructureStorage.Resolver)),
	wire.Bind(new(applicationVideo.UploadGarbageScanner), new(*infrastructureStorage.UploadGarbageScanner)),
)

// VideoProviderSet 装配视频用例、写侧仓储、页面查询、缓存和弹幕入口。
// ScheduleStore 绑定到 persistence/video 的 MySQL 实现；实时弹幕 Hub 由 NewDanmuHub 打断构造环。
var VideoProviderSet = wire.NewSet(
	// internal/application/video
	applicationVideo.ProviderSet,
	infrastructureExternalVideo.NewHTTPDASHVariantResolver,
	wire.Bind(new(applicationVideo.DASHVariantResolver), new(*infrastructureExternalVideo.HTTPDASHVariantResolver)),
	// internal/infrastructure/persistence/video
	infrastructurePersistenceVideo.NewAdminVideoQuery,
	infrastructurePersistenceVideo.NewAuthorRenameRepository,
	infrastructurePersistenceVideo.NewCoinRepository,
	infrastructurePersistenceVideo.NewDanmuQuery,
	infrastructurePersistenceVideo.NewDanmuRepository,
	infrastructurePersistenceVideo.NewDetailQuery,
	infrastructurePersistenceVideo.NewFeedQuery,
	infrastructurePersistenceVideo.NewFolderQuery,
	infrastructurePersistenceVideo.NewFolderRepository,
	infrastructurePersistenceVideo.NewHistoryQuery,
	infrastructurePersistenceVideo.NewHistoryRepository,
	infrastructurePersistenceVideo.NewLikeRepository,
	infrastructurePersistenceVideo.NewNotifySourceQuery,
	infrastructurePersistenceVideo.NewPartitionRepository,
	infrastructurePersistenceVideo.NewPinRepository,
	infrastructurePersistenceVideo.NewRecommendQuery,
	infrastructurePersistenceVideo.NewReportQuery,
	infrastructurePersistenceVideo.NewReportRepository,
	infrastructurePersistenceVideo.NewUserVideoQuery,
	infrastructurePersistenceVideo.NewVideoRepository,
	infrastructurePersistenceVideo.NewScheduleStore,
	wire.Bind(new(applicationVideo.AdminVideoQuery), new(*infrastructurePersistenceVideo.AdminVideoQuery)),
	wire.Bind(new(applicationVideo.DanmuQuery), new(*infrastructurePersistenceVideo.DanmuQuery)),
	wire.Bind(new(domainVideo.DanmuRepository), new(*infrastructurePersistenceVideo.DanmuRepository)),
	wire.Bind(new(applicationVideo.VideoDetailQuery), new(*infrastructurePersistenceVideo.DetailQuery)),
	wire.Bind(new(applicationVideo.FeedQuery), new(*infrastructurePersistenceVideo.FeedQuery)),
	wire.Bind(new(applicationVideo.FolderQuery), new(*infrastructurePersistenceVideo.FolderQuery)),
	wire.Bind(new(domainVideo.FolderRepository), new(*infrastructurePersistenceVideo.FolderRepository)),
	wire.Bind(new(applicationVideo.HistoryQuery), new(*infrastructurePersistenceVideo.HistoryQuery)),
	wire.Bind(new(domainVideo.HistoryRepository), new(*infrastructurePersistenceVideo.HistoryRepository)),
	wire.Bind(new(domainVideo.LikeRepository), new(*infrastructurePersistenceVideo.LikeRepository)),
	wire.Bind(new(domainVideo.CoinRepository), new(*infrastructurePersistenceVideo.CoinRepository)),
	wire.Bind(new(applicationVideo.NotifySourceQuery), new(*infrastructurePersistenceVideo.NotifySourceQuery)),
	wire.Bind(new(domainVideo.PartitionRepository), new(*infrastructurePersistenceVideo.PartitionRepository)),
	wire.Bind(new(applicationVideo.RecommendQuery), new(*infrastructurePersistenceVideo.RecommendQuery)),
	wire.Bind(new(applicationVideo.ReportQuery), new(*infrastructurePersistenceVideo.ReportQuery)),
	wire.Bind(new(domainVideo.ReportRepository), new(*infrastructurePersistenceVideo.ReportRepository)),
	wire.Bind(new(applicationVideo.UserVideoQuery), new(*infrastructurePersistenceVideo.UserVideoQuery)),
	wire.Bind(new(domainVideo.VideoRepository), new(*infrastructurePersistenceVideo.VideoRepository)),
	// internal/infrastructure/cache/video
	infrastructureCacheVideo.NewUploadQuotaStore,
	infrastructureCacheVideo.NewUploadHashGuard,
	wire.Bind(new(applicationVideo.UploadQuotaStore), new(*infrastructureCacheVideo.UploadQuotaStore)),
	wire.Bind(new(applicationVideo.UploadHashGuard), new(*infrastructureCacheVideo.UploadHashGuard)),
	infrastructureCacheVideo.NewAuthorIDCache,
	infrastructureCacheVideo.NewAuthorStatsCache,
	infrastructureCacheVideo.NewDanmuEventBus,
	infrastructureCacheVideo.NewDanmuRecentCache,
	infrastructureCacheVideo.NewDetailMetaCache,
	infrastructureCacheVideo.NewFolderListCache,
	infrastructureCacheVideo.NewHotRankCache,
	infrastructureCacheVideo.NewPartitionCache,
	infrastructureCacheVideo.NewStatsCache,
	infrastructureCacheVideo.NewUsernameCache,
	wire.Bind(new(applicationVideo.AuthorIDCache), new(*infrastructureCacheVideo.AuthorIDCache)),
	wire.Bind(new(applicationVideo.DanmuRecentCache), new(*infrastructureCacheVideo.DanmuRecentCache)),
	wire.Bind(new(applicationVideo.DetailMetaCache), new(*infrastructureCacheVideo.DetailMetaCache)),
	wire.Bind(new(applicationVideo.FolderListCache), new(*infrastructureCacheVideo.FolderListCache)),
	wire.Bind(new(applicationVideo.HotRankCache), new(*infrastructureCacheVideo.HotRankCache)),
	wire.Bind(new(applicationVideo.AuthorStatsCache), new(*infrastructureCacheVideo.AuthorStatsCache)),
	wire.Bind(new(applicationVideo.UsernameCache), new(*infrastructureCacheVideo.UsernameCache)),
	wire.Bind(new(applicationVideo.PartitionCache), new(*infrastructureCacheVideo.PartitionCache)),
	wire.Bind(new(applicationVideo.ScheduleStore), new(*infrastructurePersistenceVideo.ScheduleStore)),
	wire.Bind(new(applicationVideo.StatsCache), new(*infrastructureCacheVideo.StatsCache)),
	// internal/interfaces/ws/danmu
	NewDanmuHub,
	wire.Bind(new(applicationVideo.DanmuBroadcaster), new(*interfacesWsDanmu.Hub)),
	wire.Bind(new(interfacesWsDanmu.EventBus), new(*infrastructureCacheVideo.DanmuEventBus)),
	// internal/interfaces/http/handler/video
	interfacesHttpHandlerVideo.NewAdminHandler,
	interfacesHttpHandlerVideo.NewHandler,
)

// PlatformProviderSet 装配周期维护用例与调度所需的队列、快照、投影适配器。
// CronComponent 消费各 Job 端口，触发器不直接实现领域计算。
var PlatformProviderSet = wire.NewSet(
	// internal/application/platform
	applicationPlatform.ProviderSet,
	// internal/infrastructure/scheduler
	infrastructureScheduler.NewBanMaintenanceRepository,
	infrastructureScheduler.NewCommentLikeProjection,
	infrastructureScheduler.NewCommentLikeQueue,
	infrastructureScheduler.NewCronComponent,
	infrastructureScheduler.NewHotRankStore,
	infrastructureScheduler.NewVideoStatsCache,
	infrastructureScheduler.NewVideoStatsProjection,
	wire.Bind(new(applicationPlatform.BanMaintenanceRepository), new(*infrastructureScheduler.BanMaintenanceRepository)),
	wire.Bind(new(applicationPlatform.CommentLikeQueue), new(*infrastructureScheduler.CommentLikeQueue)),
	wire.Bind(new(applicationPlatform.CommentLikeProjection), new(*infrastructureScheduler.CommentLikeProjection)),
	wire.Bind(new(applicationPlatform.HotRankStore), new(*infrastructureScheduler.HotRankStore)),
	wire.Bind(new(applicationPlatform.VideoStatsCache), new(*infrastructureScheduler.VideoStatsCache)),
	wire.Bind(new(applicationPlatform.VideoStatsProjection), new(*infrastructureScheduler.VideoStatsProjection)),
	wire.Bind(new(infrastructureScheduler.CommentLikeSyncJob), new(*applicationPlatform.CommentLikeSyncService)),
	wire.Bind(new(infrastructureScheduler.VideoStatSyncJob), new(*applicationPlatform.VideoStatSyncService)),
	wire.Bind(new(infrastructureScheduler.HotRankDecayJob), new(*applicationPlatform.HotRankDecayService)),
	wire.Bind(new(infrastructureScheduler.AutoUnbanJob), new(*applicationPlatform.AutoUnbanService)),
)

// IdentityHTTPProviderSet 装配身份 HTTP 处理器及其 OAuth、发信和邮箱验证能力。
// 核心账号仓储/服务由 IdentityProviderSet 提供，此处补足面向请求入口的外部依赖。
var IdentityHTTPProviderSet = wire.NewSet(
	// internal/infrastructure/external/oauth
	infrastructureExternalOauth.NewBodyReader,
	infrastructureExternalOauth.NewGitHubClient,
	infrastructureExternalOauth.NewGoogleClient,
	infrastructureExternalOauth.NewLinuxDoClient,
	infrastructureExternalOauth.NewQQClient,
	infrastructureExternalOauth.NewStateStore,
	infrastructureExternalOauth.NewXClient,
	wire.Bind(new(applicationIdentity.GitHubOAuthClient), new(*infrastructureExternalOauth.GitHubClient)),
	wire.Bind(new(applicationIdentity.GoogleOAuthClient), new(*infrastructureExternalOauth.GoogleClient)),
	wire.Bind(new(applicationIdentity.LinuxDoOAuthClient), new(*infrastructureExternalOauth.LinuxDoClient)),
	wire.Bind(new(applicationIdentity.QQOAuthClient), new(*infrastructureExternalOauth.QQClient)),
	wire.Bind(new(applicationIdentity.OAuthStateStore), new(*infrastructureExternalOauth.StateStore)),
	wire.Bind(new(applicationIdentity.XOAuthClient), new(*infrastructureExternalOauth.XClient)),
	// internal/infrastructure/external/email
	infrastructureExternalEmail.NewSender,
	infrastructureExternalEmail.NewAddressVerifier,
	wire.Bind(new(applicationIdentity.EmailCaptchaSender), new(*infrastructureExternalEmail.Sender)),
	wire.Bind(new(applicationIdentity.EmailAddressVerifier), new(*infrastructureExternalEmail.AddressVerifier)),
	// internal/interfaces/http/handler/identity
	interfacesHttpHandlerIdentity.NewAdminHandler,
	interfacesHttpHandlerIdentity.NewCaptchaHandler,
	interfacesHttpHandlerIdentity.NewHandler,
)

// NotificationHTTPProviderSet 通知上下文的 HTTP 处理器 provider 集合。
var NotificationHTTPProviderSet = wire.NewSet(
	// internal/interfaces/http/handler/notification
	interfacesHttpHandlerNotification.NewHandler,
)

// SearchIndexProviderSet 装配关键词查询、用户文档和视频向量索引的 ES 实现。
// 同一 Adapter 满足搜索、提及和联想接口，VideoVectorIndex 同时提供读写与生命周期能力。
var SearchIndexProviderSet = wire.NewSet(
	// internal/infrastructure/search
	infrastructureSearch.NewAdapter,
	wire.Bind(new(applicationSearch.KeywordSearch), new(*infrastructureSearch.Adapter)),
	wire.Bind(new(applicationSearch.MentionSearch), new(*infrastructureSearch.Adapter)),
	wire.Bind(new(applicationSearch.SuggestSearch), new(*infrastructureSearch.Adapter)),
	infrastructureSearch.NewIndexManager,
	infrastructureSearch.NewUserIndexer,
	infrastructureSearch.NewVideoVectorIndex,
	wire.Bind(new(applicationSearch.UserDocumentIndexer), new(*infrastructureSearch.UserIndexer)),
	wire.Bind(new(applicationSearch.VideoVectorIndex), new(*infrastructureSearch.VideoVectorIndex)),
	wire.Bind(new(applicationSearch.VideoVectorIndexLifecycle), new(*infrastructureSearch.VideoVectorIndex)),
)

// WebSocketProviderSet 是两个 WebSocket 端点共享的来源校验器。
var WebSocketProviderSet = wire.NewSet(
	interfacesWsOrigin.NewChecker,
)
