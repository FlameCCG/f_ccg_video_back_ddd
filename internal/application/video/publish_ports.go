package video

import (
	"context"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// TranscodeDispatcher 转码任务投递端口，绑定到 media 的任务生产者，使用规定的 Kafka topic 与消息结构。
type TranscodeDispatcher interface {
	// PublishTranscodeTask 投递一条转码任务。
	PublishTranscodeTask(ctx context.Context, task TranscodeTask) error
	// MarkTranscodeQueued 把任务标记为「已入队」，供管理端进度查询展示。
	MarkTranscodeQueued(ctx context.Context, videoID, partID uint)
}

// TranscodeTask 转码任务载荷。
type TranscodeTask struct {
	ResolutionList []uint16
	StorageType    enum.VideoStorageType
	SourcePath     string
	OutputPath     string
	FileHash       string
	FileName       string
	VideoID        uint
	VideoPartID    uint
}
