package cache

import "github.com/google/wire"

// ProviderSet 汇总 Redis 客户端及其底层连接。
var ProviderSet = wire.NewSet(
	Provide,
	ProvideClient,
)
