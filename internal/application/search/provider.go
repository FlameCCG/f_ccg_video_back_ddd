package search

import "github.com/google/wire"

// ProviderSet 汇总搜索上下文的应用服务构造器。
var ProviderSet = wire.NewSet(
	NewHotKeywordService,
	NewMentionService,
	NewSearchService,
	NewSuggestService,
	NewUserIndexService,
	NewVideoVectorRebuildService,
	NewVideoVectorService,
)
