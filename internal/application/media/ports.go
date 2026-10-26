// Package media 编排媒体任务派发、转码管线、进度查询与取消。
//
// 从 transcode_service.go 的 ProcessTask 读完整流程，再看 pipeline_coordinator.go 的 MP4/DASH 分支。
// 领域根据源片元信息和策略生成计划；本层调度探测、编码、检查点、上传及资源保存端口。
// 错误分类会影响消息确认：永久错误走死信，删除使任务作废，可重试错误保留恢复机会。
// 数据库资源行、存储对象和检查点要分别核验，单凭“进度为 100%”不能替代产物有效性检查。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package media

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/media"
)

// -----------------------------------------------------------------------------
// 基础设施端口（实现位于 internal/infrastructure/mq）
// -----------------------------------------------------------------------------

// 端口阅读：本文件描述探测、编码、源文件暂存与检查点等应用编排能力；存储、资源和进度的领域端口位于 domain/media/ports.go。
// 引擎可同时实现多个窄接口，应用只持有自己所需能力，具体选择由组合根完成。

// Prober 媒体元信息探测端口（ffprobe + 源文件体积探测）。
//
// 实现：`*mq.TranscodeEngine`（`internal/infrastructure/mq`）。
type Prober interface {
	// ProbeVideo 探测本地媒体文件的分辨率、码率、编码、容器 brand 与时长。
	ProbeVideo(ctx context.Context, path string) (domain.VideoMeta, error)
	// SourceSize 返回任务源文件字节数：本地源用 os.Stat，远程源用 HTTP HEAD。
	// 取不到不致命，调用方按 0 处理。
	SourceSize(ctx context.Context, task domain.TranscodeTask) (int64, error)
}

// Checkpoint 是一次转码任务的断点目录句柄（对应用层不透明）。
//
// 实现：`internal/infrastructure/mq` 的 `*transcodeCheckpoint`。
type Checkpoint interface {
	// Persistent 报告断点目录是否可跨尝试复用；为 false 时分段转码没有意义。
	Persistent() bool
	// Remove 删除整个断点目录（任务成功 / 视频删除 / 永久失败时调用）。
	Remove()
}

// CheckpointManager 断点目录管理端口。
//
// 实现：`*mq.TranscodeEngine`。
type CheckpointManager interface {
	// OpenCheckpoint 打开（或创建）任务的断点目录；断点续传被关闭时退化为一次性临时目录。
	OpenCheckpoint(ctx context.Context, task domain.TranscodeTask) (Checkpoint, error)
	// StartCheckpointJanitor 启动孤儿断点目录清扫：立即扫一次，此后周期扫，随 ctx 退出。
	StartCheckpointJanitor(ctx context.Context)
}

// SourceStager 转码源文件准备端口。
//
// 实现：`*mq.TranscodeEngine`。本地源校验存在性后直接返回原路径；
// 远程源下载到断点目录并原子提交，重试可复用（cleanup 由断点目录生命周期接管）。
type SourceStager interface {
	// PrepareSource 返回可供 ffmpeg 读取的本地路径与清理函数。
	PrepareSource(
		ctx context.Context,
		task domain.TranscodeTask,
		ckpt Checkpoint,
	) (localPath string, cleanup func(), err error)
}

// ArtifactStateChecker 「产物是否已完整入库」判定端口（断点续传 L1）。
//
// 判定同时需要 DB 资源行与远端对象 Stat，因此实现落在 infrastructure
// （`*mq.TranscodeEngine` 持有资源仓储）。任何查询/Stat 失败都按「未完成」处理。
type ArtifactStateChecker interface {
	// TargetPersisted 判断 MP4 target 是否已入库：资源行 file_url 指向本任务期望产物
	// 且远端对象存在。命中时返回该资源行的 file_url。
	TargetPersisted(
		ctx context.Context,
		task domain.TranscodeTask,
		target domain.TranscodeTarget,
		driver domain.StorageDriver,
	) (string, bool)
	// DASHPersisted 判断整套 DASH 是否已入库：自适应资源行指向本任务 manifest 且对象存在。
	DASHPersisted(ctx context.Context, task domain.TranscodeTask, driver domain.StorageDriver) bool
}

// MP4EncodeRequest 是一批 MP4 输出目标的编码请求。
//
// 断点复用（L2）、分段转码与产物提交全部由适配器内部完成：
// 返回值覆盖 Targets 中的每一个目标，值为断点目录中已提交的本地产物路径。
type MP4EncodeRequest struct {
	// Task 本次转码任务。
	Task domain.TranscodeTask
	// Policy 编码策略（由配置转换而来）。
	Policy *domain.TranscodePolicy
	// Checkpoint 任务断点目录句柄。
	Checkpoint Checkpoint
	// SourcePath 已准备好的本地源文件路径。
	SourcePath string
	// Targets 本次要产出的全部 MP4 目标（已剔除 L1 已入库的目标）。
	Targets []domain.TranscodeTarget
	// UseGPU 是否使用 NVENC 编码。
	UseGPU bool
	// SourceBitrate 源码率(kbps)，用于 -maxrate 预算。
	SourceBitrate int
	// SourceDuration 源片时长(秒)，用于超时预算与产物时长校验。
	SourceDuration int
	// SourceCodec 源视频编码名（ffprobe），用于高效源码率抬升。
	SourceCodec string
	// HasAudio 源是否带音轨。
	HasAudio bool
	// ChunkPlan 分段计划；为空表示单次直跑。
	ChunkPlan []domain.TranscodeChunk
}

// DASHEncodeRequest 是整套 DASH 的编码请求。
//
// 断点复用、分段转码、产物 probe/时长校验与整目录原子提交由适配器内部完成；
// 返回值为断点目录中已提交的 DASH 目录（其中含 manifest.mpd 与各 representation）。
type DASHEncodeRequest struct {
	// Task 本次转码任务。
	Task domain.TranscodeTask
	// Policy 编码策略。
	Policy *domain.TranscodePolicy
	// Checkpoint 任务断点目录句柄。
	Checkpoint Checkpoint
	// SourcePath 已准备好的本地源文件路径。
	SourcePath string
	// Targets 各 representation 目标。
	Targets []domain.TranscodeTarget
	// UseGPU 是否使用 NVENC 编码。
	UseGPU bool
	// SourceBitrate 源码率(kbps)。
	SourceBitrate int
	// SourceDuration 源片时长(秒)。
	SourceDuration int
	// SourceCodec 源视频编码名。
	SourceCodec string
	// HasAudio 源是否带音轨（决定 adaptation_sets 是否含音频）。
	HasAudio bool
	// ChunkPlan 分段计划；为空表示单次直跑。
	ChunkPlan []domain.TranscodeChunk
}

// TranscodeEncoder FFmpeg 编码执行端口。
//
// 实现：`*mq.TranscodeEngine`。ffmpeg 命令行参数、断点提交与分段拼装全在 infrastructure，
// 应用层只负责「跑哪些目标、GPU 还是 CPU、产物如何上传落库」。
type TranscodeEncoder interface {
	// EncodeMP4Targets 产出（或复用）各 MP4 目标的本地产物，返回 targetID → 本地路径。
	EncodeMP4Targets(ctx context.Context, req MP4EncodeRequest) (map[string]string, error)
	// EncodeDASHDir 产出（或复用）整套 DASH，返回已提交的本地目录。
	EncodeDASHDir(ctx context.Context, req DASHEncodeRequest) (string, error)
}

// -----------------------------------------------------------------------------
// 跨上下文端口
// -----------------------------------------------------------------------------
//
// 转码管线在原实现（service/mq_service、service/storage_service、
// service/transcode_cancel_service）中不向任何其它模块发起出向调用：
// 它只读写 videos / video_parts / video_resources / video_part_resources 四张表
// （经领域端口 domain.TranscodeResourceRepository）、写 Redis 进度与详情缓存
// （domain.ProgressStore / domain.VideoDetailCacheInvalidator）、
// 读写对象存储（domain.StorageDriver）并广播取消事件（domain.CancelBroadcaster）。
//
// 因此 media 上下文**没有**需要绑定到其它上下文能力的出向端口：
// 既不发通知、不更新搜索索引，也不修改 videos.status（原实现只读 status 判断是否被硬删除）。
// 反向的入向能力（video 上下文要用的投递/查进度/取消/存储）由本包的
// TranscodeDispatchService / ProgressQueryService / CancelService / StorageService 导出。
