package mysql

import "github.com/google/wire"

// ProviderSet 汇总 MySQL 客户端及其 GORM 连接。
var ProviderSet = wire.NewSet(
	Provide,
	ProvideGorm,
)
