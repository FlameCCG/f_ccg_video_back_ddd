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
var TranscodeWorkerProviderSet = wire.NewSet(
	CLIDatabaseProviderSet,
	cache.ProviderSet,
	gormx.NewTxManager,
	wire.Bind(new(tx.Manager), new(*gormx.TxManager)),
	ctxutil.NewDetachedTaskGroup,
	MediaProviderSet,
	NewTranscodeWorker,
)
