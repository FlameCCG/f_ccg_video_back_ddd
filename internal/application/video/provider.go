package video

import "github.com/google/wire"

// ProviderSet 汇总视频上下文的应用服务构造器。
var ProviderSet = wire.NewSet(
	NewAdminVideoService,
	NewAuthorService,
	NewAuthorStatsService,
	NewDanmuService,
	NewDeleteService,
	NewDetailService,
	NewFeedService,
	NewFolderService,
	NewHistoryService,
	NewHTTPDASHVariantResolver,
	NewInteractionService,
	NewPartitionService,
	NewPublishService,
	NewRecommendService,
	NewReportService,
	NewScheduler,
	NewStatsService,
	NewUpdateService,
	NewUploadService,
	NewUploadQuotaService,
	NewUploadCleanupService,
	NewUserVideoService,
	NewVectorSyncService,
)
