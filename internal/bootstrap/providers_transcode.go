package bootstrap

import (
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/internal/infrastructure/cache"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/google/wire"
)

// TranscodeWorkerProviderSet 只装配转码所需的 MySQL、Redis、Kafka 和存储。
// Wire 从 MediaProviderSet 中只保留转码消费实际引用的构造器。
//
// 独立 worker 与完整 App 使用相同应用用例，但注入器的输出类型不同，因此可达依赖也不同。
// 不要为复用某个服务直接调用 InitializeApp，否则会连带装配 HTTP、搜索或其他后台组件。
var TranscodeWorkerProviderSet = wire.NewSet(
	CLIDatabaseProviderSet,
	cache.ProviderSet,
	gormx.NewTxManager,
	wire.Bind(new(tx.Manager), new(*gormx.TxManager)),
	ctxutil.NewDetachedTaskGroup,
	MediaProviderSet,
	NewTranscodeWorker,
)
