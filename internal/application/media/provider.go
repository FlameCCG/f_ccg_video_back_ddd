package media

import (
	domain "f_ccg_video_back_ddd/internal/domain/media"

	"github.com/google/wire"
)

// ProviderSet 汇总 media 上下文的应用构造器，数据库、缓存与外部服务实现由 bootstrap 绑定，测试可注入 fake。
//
// GPU 冷却窗口与任务取消登记表在此创建为进程内唯一实例。仓储、缓存、存储与编码器端口的 wire.Bind 由 bootstrap 汇总。
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
