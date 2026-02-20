package chat

import "github.com/google/wire"

// ProviderSet 汇总聊天上下文的应用服务构造器。
var ProviderSet = wire.NewSet(
	NewService,
)
