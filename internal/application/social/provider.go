package social

import "github.com/google/wire"

// ProviderSet 汇总社交上下文的应用服务构造器。
var ProviderSet = wire.NewSet(
	NewFocusService,
	NewRelationService,
)
