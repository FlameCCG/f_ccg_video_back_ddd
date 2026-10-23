package media

import (
	"context"
	"io"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// ChunkReader 是分片写入时的数据读取接口（原 storage_service.ChunkReader）。
// 声明为别名，使跨上下文端口可以直接用 io.Reader 声明同名方法。
type ChunkReader = io.Reader

// StorageDriver 抽象分片上传、合并与对象读写的存储驱动（原 storage_service.Driver）。
// 实现位于 internal/infrastructure/storage（本地磁盘 / MinIO）。
type StorageDriver interface {
	// SaveChunk 写入一个上传分片。
	SaveChunk(ctx context.Context, fileHash, index string, size int64, reader ChunkReader) error
	// ListUploadedChunks 列出指定文件 hash 已上传的分片索引。
	ListUploadedChunks(ctx context.Context, fileHash string) ([]string, error)
	// GetCompletedFile 检查最终合并文件是否已存在，并返回其路径/URL。
	GetCompletedFile(ctx context.Context, fileHash string) (string, bool, error)
	// MergeChunks 把顺序分片合并成最终文件。
	MergeChunks(ctx context.Context, fileHash, fileName string, totalChunks int) (string, error)
	// SaveFile 写入任意文件到指定相对路径。
	SaveFile(ctx context.Context, relativePath string, size int64, reader io.Reader, contentType string) (string, error)
	// SaveTranscodeVideo 流式写入转码产物，返回访问路径与实际字节数。
	SaveTranscodeVideo(
		ctx context.Context,
		relativePath string,
		size int64,
		reader io.Reader,
		contentType, label string,
	) (string, int64, error)
	// ResolveSource 把存储中的相对路径/对象 key 解析为 ffmpeg 可直接读取的
	// 本地绝对路径（本地存储）或可访问 URL（MinIO）。已持有绝对形式时原样返回。
	ResolveSource(ctx context.Context, storedPath string) (string, error)
	// StatStoredFile 返回已存储文件的字节数；文件不存在或不可访问时返回 error。
	// 供转码断点续传在复用历史产物前校验远端对象仍然有效。
	StatStoredFile(ctx context.Context, storedPath string) (int64, error)
	// DeleteStoredFile 删除已存储对象；空路径视为 no-op。
	DeleteStoredFile(ctx context.Context, storedPath string) error
	// DeleteByPrefix 删除指定前缀下的所有文件，返回删除数量。
	// 用于 DASH 等「单个 DB 记录对应多个存储文件」的场景。
	DeleteByPrefix(ctx context.Context, prefix string) (int64, error)
}

// StorageResolver 依据站点配置解析存储驱动。
// 实现位于 internal/infrastructure/storage。
type StorageResolver interface {
	// Resolve 按站点配置选择本地或 MinIO 驱动。
	Resolve() (StorageDriver, error)
	// ResolveByType 按显式存储类型选择驱动；未知类型退回 Resolve。
	ResolveByType(storageType enum.VideoStorageType) (StorageDriver, error)
	// CheckStorageTyped 返回当前站点启用的存储类型。
	CheckStorageTyped() enum.VideoStorageType
	// ValidateUploadID 校验上传标识，阻止路径穿越与非法对象键。
	ValidateUploadID(uploadID string) error
	// ValidateChunkIndex 校验分片索引。
	ValidateChunkIndex(index string) error
}

// TranscodeResourceRepository 转码任务的数据库读写端口。
//
// 这些表（videos / video_parts / video_resources / video_part_resources）在领域划分上
// 属于 video 上下文，但原实现由 mq_service 直接读写；为保持外部行为 1:1，
// 实现仍放在 internal/infrastructure/persistence/media。
type TranscodeResourceRepository interface {
	// EnsureTaskActive 校验视频（及分P）仍存在且未被删除；已删除时返回 ErrVideoDeleted。
	EnsureTaskActive(ctx context.Context, task TranscodeTask) error
	// UpdateDurationFromProbe 用探测到的时长回填分P/视频时长。
	UpdateDurationFromProbe(ctx context.Context, task TranscodeTask, duration int) error
	// SaveOriginalResource 落库源视频资源行（同分辨率去重、保留已规范化产物 URL）。
	SaveOriginalResource(
		ctx context.Context,
		task TranscodeTask,
		meta VideoMeta,
		sourceURL string,
		size int64,
		highBitrate bool,
	) error
	// UpdateOriginalResourceFile 把源资源行指向规范化后的产物。
	UpdateOriginalResourceFile(
		ctx context.Context,
		task TranscodeTask,
		res enum.VideoResolution,
		newFileName, url string,
		size int64,
		bitrate int,
	) error
	// SaveVideoResource 按输出高度 upsert 一条派生资源行。
	SaveVideoResource(
		ctx context.Context,
		task TranscodeTask,
		res uint16,
		sourceName, url string,
		size int64,
		bitrate int,
	) error
	// SaveSpecificVideoResource 按显式分辨率枚举 upsert 一条资源行。
	SaveSpecificVideoResource(
		ctx context.Context,
		task TranscodeTask,
		res enum.VideoResolution,
		sourceName, url string,
		size int64,
		bitrate int,
		isVip, isSource bool,
	) error
	// SaveDASHResource 把整套 DASH 落库为一行自适应资源。
	SaveDASHResource(ctx context.Context, task TranscodeTask, sourceName, mpdURL string, totalSize int64) error
	// PersistedResourceURLs 返回匹配 (resolution, isSource) 的资源行 file_url 列表；
	// resolution 为空串表示不按分辨率过滤（源行查询用）。
	PersistedResourceURLs(
		ctx context.Context,
		task TranscodeTask,
		resolution enum.VideoResolution,
		isSource bool,
	) ([]string, error)
}

// ProgressStore 转码进度写端口（Redis 适配器实现）。
type ProgressStore interface {
	// Write 写入进度快照并广播变更；ttl 由调用方按终态与否给出。
	Write(ctx context.Context, snapshot *TranscodeProgress, ttl time.Duration) error
	// Clear 删除进度快照（视频被删除时调用）。
	Clear(ctx context.Context, videoID, partID uint)
}

// ProgressReader 转码进度读端口（管理端快照与 SSE 用）。
type ProgressReader interface {
	// Get 读取单条进度；不存在时返回 (nil, nil)。
	Get(ctx context.Context, videoID, partID uint) (*TranscodeProgress, error)
	// ByVideoIDs 批量读取视频维度进度（有分P 时展开为各分P 进度）。
	ByVideoIDs(ctx context.Context, videoIDs []uint) ([]TranscodeProgress, error)
	// ByPairs 按显式 (videoId, partId) 组合批量读取进度。
	ByPairs(ctx context.Context, pairs [][2]uint) ([]TranscodeProgress, error)
	// Subscribe 订阅进度变更；调用方负责 Close。
	Subscribe(ctx context.Context) (ProgressSubscription, error)
}

// ProgressSubscription 进度变更订阅句柄。
type ProgressSubscription interface {
	// Events 返回进度快照通道；通道关闭表示订阅结束。
	Events() <-chan TranscodeProgress
	// Close 关闭订阅。
	Close() error
}

// VideoDetailCacheInvalidator 视频详情 meta 缓存失效端口。
//
// 转码/资源变更后必须让详情缓存重建；原实现直接操作 Redis key 以避免 mq_service
// 与 video_service 的循环依赖，新架构改为端口 + cache/media 适配器。
type VideoDetailCacheInvalidator interface {
	// InvalidateVideoDetailMeta 删除 meta 缓存并 bump generation，阻止在途旧快照回填。
	InvalidateVideoDetailMeta(ctx context.Context, videoID, partID uint)
}

// CancelBroadcaster 转码取消事件广播端口（Redis Pub/Sub 适配器实现）。
type CancelBroadcaster interface {
	// PublishVideoDeleted 向其它实例广播「这些视频已被硬删除」。
	PublishVideoDeleted(ctx context.Context, videoIDs []uint) error
	// Available 报告底层广播通道是否可用（不可用时只做本地取消）。
	Available() bool
}

// TaskPublisher 转码任务投递端口（Kafka 适配器实现）。
type TaskPublisher interface {
	// PublishTranscodeTask 把任务序列化后投递到转码 topic。
	PublishTranscodeTask(ctx context.Context, task TranscodeTask) error
}
