package dynamic

import "github.com/google/wire"

// ProviderSet 汇总动态上下文的应用服务构造器。
var ProviderSet = wire.NewSet(
	NewAdminService,
	NewFeedService,
	NewFollowService,
	NewLikeService,
	NewReadService,
	NewService,
)
