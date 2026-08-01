package contentops

import "github.com/google/wire"

// ProviderSet 汇总内容运营上下文的应用服务构造器。
var ProviderSet = wire.NewSet(
	NewBannerService,
	NewOAuthURLService,
	NewSiteConfigService,
	NewStatService,
)
