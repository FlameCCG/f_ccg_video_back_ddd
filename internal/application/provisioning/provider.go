package provisioning

import "github.com/google/wire"

// ProviderSet 汇总初始化任务上下文的应用服务构造器。
var ProviderSet = wire.NewSet(
	NewBannerService,
	NewSuperUserService,
)
