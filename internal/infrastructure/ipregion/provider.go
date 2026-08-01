package ipregion

import "github.com/google/wire"

// ProviderSet 汇总 IP 地域检索基础设施构造器。
var ProviderSet = wire.NewSet(
	Provide,
	ProvideXDB,
	NewResolver,
)
