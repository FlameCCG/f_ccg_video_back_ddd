package media

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/media"
)

// TranscodeDispatchService 投递转码任务并标记「已入队」进度
// （原 mq_service.MQService.PublishTranscodeTask + MarkTranscodeQueued 的调用组合）。
//
// video 上下文的投稿流程经此服务把任务丢进 Kafka；
// Kafka topic 名与消息 JSON 结构由 infrastructure/mq 的 TranscodePayload 保证 1:1。
type TranscodeDispatchService struct {
	publisher domain.TaskPublisher
	progress  domain.ProgressStore
}

// NewTranscodeDispatchService 构造转码任务投递服务。
func NewTranscodeDispatchService(
	publisher domain.TaskPublisher,
	progress domain.ProgressStore,
) *TranscodeDispatchService {
	return &TranscodeDispatchService{publisher: publisher, progress: progress}
}

// PublishTranscodeTask 投递一条转码任务到异步转码队列。
func (s *TranscodeDispatchService) PublishTranscodeTask(ctx context.Context, input TranscodeTaskInput) error {
	return s.publisher.PublishTranscodeTask(ctx, input.task())
}

// MarkTranscodeQueued 写入「排队等待转码」进度快照，供管理端进度查询展示。
//
// 与原 MarkTranscodeQueued 一致：videoID 为 0 时直接返回，写入失败只被忽略（最佳努力）。
func (s *TranscodeDispatchService) MarkTranscodeQueued(ctx context.Context, videoID, partID uint) {
	if s == nil || s.progress == nil || videoID == 0 {
		return
	}

	snapshot := domain.NewQueuedProgress(videoID, partID)
	// 原实现对 queued 快照固定使用运行中 TTL（2 小时），此处逐字保留。
	_ = s.progress.Write(ctx, snapshot, domain.TranscodeProgressRunningTTL)
}
