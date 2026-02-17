package mongodb

import "github.com/google/wire"

// ProviderSet 汇总 MongoDB 客户端及其数据库句柄。
var ProviderSet = wire.NewSet(
	Provide,
	ProvideDatabase,
)
