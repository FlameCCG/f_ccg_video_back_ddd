package media

import (
	domain "f_ccg_video_back_ddd/internal/domain/media"

	"github.com/google/wire"
)

// ProviderSet 是 media 上下文的应用层 provider 集合。
//
// 领域侧的进程级共享状态（GPU 冷却窗口、任务取消登记表）在此创建为唯一实例：
// 原实现分别是 mq_service 的包级变量 gpuUnavailableUntil
// 与 transcode_cancel_service.DefaultRegistry。
//
// 仓储/缓存/存储/编码器等端口的 wire.Bind 由 bootstrap 汇总（实现在 infrastructure）。
var ProviderSet = wire.NewSet(
	NewTranscodeService,
	NewTranscodeDispatchService,
	NewProgressQueryService,
	NewCancelService,
	NewStorageService,
	ProvideGPUCooldown,
	ProvideTaskRegistry,
)

// ProvideGPUCooldown 创建进程内唯一的 GPU 编码冷却窗口。
func ProvideGPUCooldown() *domain.GPUCooldown {
	return &domain.GPUCooldown{}
}

// ProvideTaskRegistry 创建进程内唯一的转码任务取消登记表。
func ProvideTaskRegistry() *domain.TaskRegistry {
	return domain.NewTaskRegistry()
}
