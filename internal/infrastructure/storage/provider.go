package storage

import "github.com/google/wire"

// ProviderSet 汇总本地与 MinIO 存储基础设施构造器。
var ProviderSet = wire.NewSet(
	NewLocalStorage,
	NewMinIOClientPool,
	NewMinIOStorage,
	NewUploadGarbageScanner,
	NewResolver,
)
