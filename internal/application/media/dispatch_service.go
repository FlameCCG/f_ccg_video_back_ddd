package media

import (
	"context"
	mediaruntime "f_ccg_video_back_ddd/internal/application/media/runtime"
	"time"

	mediadomain "f_ccg_video_back_ddd/internal/domain/media"
)

// TranscodeDispatchService 投递转码任务并标记已入队进度。
//
// video 的投稿流程通过此服务发送 Kafka 任务，topic 与消息结构由 infrastructure/mq 的 TranscodePayload 定义。
type TranscodeDispatchService struct {
	publisher TaskPublisher
	progress  mediaruntime.ProgressStore
}

// NewTranscodeDispatchService 构造转码任务投递服务。
func NewTranscodeDispatchService(
	publisher TaskPublisher,
	progress mediaruntime.ProgressStore,
) *TranscodeDispatchService {
	return &TranscodeDispatchService{publisher: publisher, progress: progress}
}

// PublishTranscodeTask 投递一条转码任务到异步转码队列。
func (s *TranscodeDispatchService) PublishTranscodeTask(ctx context.Context, input TranscodeTaskInput) error {
	return s.publisher.PublishTranscodeTask(ctx, input.task())
}

// MarkTranscodeQueued 写入「排队等待转码」进度快照，供管理端进度查询展示。
//
// videoID 为 0 时直接返回，写入失败只被忽略（最佳努力）。
func (s *TranscodeDispatchService) MarkTranscodeQueued(ctx context.Context, videoID, partID uint) {
	if s == nil || s.progress == nil || videoID == 0 {
		return
	}

	snapshot := mediadomain.NewQueuedProgress(videoID, partID, time.Now())
	// queued 快照使用运行中 TTL（2 小时）。
	_ = s.progress.Write(ctx, snapshot, mediaruntime.ProgressRunningTTL)
}
