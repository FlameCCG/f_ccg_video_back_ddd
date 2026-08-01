package platform

import "github.com/google/wire"

// ProviderSet 汇总平台后台任务上下文的应用服务构造器。
var ProviderSet = wire.NewSet(
	NewAutoUnbanService,
	NewCommentLikeSyncService,
	NewHotRankDecayService,
	NewVideoStatSyncService,
)
