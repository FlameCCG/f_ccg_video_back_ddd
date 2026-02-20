package search

import "github.com/google/wire"

// ProviderSet 汇总 Elasticsearch v7/v8 基础客户端。
var ProviderSet = wire.NewSet(
	Provide,
	ProvideV7,
	ProvideV8,
)
