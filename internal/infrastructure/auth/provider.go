package auth

import "github.com/google/wire"

// ProviderSet 汇总 Casbin 鉴权基础设施构造器。
var ProviderSet = wire.NewSet(
	NewEnforcer,
)
