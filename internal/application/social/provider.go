package social

import "github.com/google/wire"

// ProviderSet 汇总社交上下文的应用服务构造器。
//
// 本集合只登记本上下文的应用构造器，不选择数据库、缓存或外部服务实现。
// bootstrap 把消费方接口绑定到具体适配器后，Wire 才能生成完整构造链；本层因此可直接注入 fake 做用例测试。
var ProviderSet = wire.NewSet(
	NewFocusService,
	NewRelationService,
)
