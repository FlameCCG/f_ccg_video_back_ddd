package comment

import "github.com/google/wire"

// ProviderSet 汇总评论上下文的应用服务构造器。
var ProviderSet = wire.NewSet(
	NewService,
)
