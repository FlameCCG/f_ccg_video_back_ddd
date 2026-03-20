package video

import (
	"context"
	"io"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// TranscodeProgressReader 转码进度查询与订阅端口。
//
// 绑定目标：media 上下文。
type TranscodeProgressReader interface {
	// MGetByVideoIDs 批量读取给定视频的进度快照。
	MGetByVideoIDs(ctx context.Context, videoIDs []uint) ([]TranscodeProgressItem, error)
	// Subscribe 订阅进度变更事件；返回的订阅需由调用方 Close。
	Subscribe(ctx context.Context) (TranscodeProgressSubscription, error)
	// IsTerminalStatus 判断进度状态是否为终态（succeeded / failed）。
	IsTerminalStatus(status string) bool
}

// TranscodeProgressSubscription 转码进度订阅。
type TranscodeProgressSubscription interface {
	// Events 返回进度事件通道；通道关闭表示订阅结束。
	Events() <-chan TranscodeProgressItem
	// Close 关闭订阅。
	Close() error
}

// StorageDriver 对象存储驱动端口。
type StorageDriver interface {
	SaveChunk(ctx context.Context, fileHash, index string, size int64, reader io.Reader) error
	// ChunkUploadBytes 返回该文件已落盘分片的总字节数，以及 index 这一片的字节数。
	// index 还没有落盘时 current 为 0。调用方用 total-current+新分片大小判断整文件是否超限。
	ChunkUploadBytes(ctx context.Context, fileHash, index string) (total, current int64, err error)
	ListUploadedChunks(ctx context.Context, fileHash string) ([]string, error)
	GetCompletedFile(ctx context.Context, fileHash string) (string, bool, error)
	MergeChunks(ctx context.Context, fileHash, fileName string, totalChunks int) (string, error)
	SaveFile(ctx context.Context, relativePath string, size int64, reader io.Reader, contentType string) (string, error)
	DeleteStoredFile(ctx context.Context, storedPath string) error
	DeleteByPrefix(ctx context.Context, prefix string) (int64, error)
}

// StorageResolver 存储驱动解析与上传标识校验端口。
//
// 绑定目标：media 上下文。
type StorageResolver interface {
	// Resolve 按站点配置解析当前生效的存储驱动。
	Resolve() (StorageDriver, error)
	// ResolveByType 按视频记录的存储类型解析驱动（硬删清理历史对象时使用）。
	ResolveByType(storageType enum.VideoStorageType) (StorageDriver, error)
	// ValidateUploadID 校验上传标识（fileHash）合法性。
	ValidateUploadID(id string) error
	// ValidateChunkIndex 校验分片索引合法性。
	ValidateChunkIndex(index string) error
}
