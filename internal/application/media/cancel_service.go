package media

import (
	"context"
	"f_ccg_video_back_ddd/pkg/logctx"

	domain "f_ccg_video_back_ddd/internal/domain/media"

	"go.uber.org/zap"
)

// CancelService 转码取消协调用例（原 service/transcode_cancel_service.NotifyVideoDeleted）。
//
// 视频被硬删除时：先同步取消本实例在途任务（登记表），再向其它实例广播；
// Redis 不可用时只做本地取消并记日志，与原实现的分支与日志文案逐字一致。
type CancelService struct {
	registry    *domain.TaskRegistry
	broadcaster domain.CancelBroadcaster
	logger      *zap.Logger
}

// NewCancelService 构造转码取消协调服务。
func NewCancelService(
	registry *domain.TaskRegistry,
	broadcaster domain.CancelBroadcaster,
	logger *zap.Logger,
) *CancelService {
	return &CancelService{registry: registry, broadcaster: broadcaster, logger: logger}
}

// NotifyVideoDeleted 取消本地在途任务并广播取消事件。
func (s *CancelService) NotifyVideoDeleted(ctx context.Context, videoIDs []uint) error {
	ids := domain.NormalizeVideoIDs(videoIDs)
	if len(ids) == 0 {
		return nil
	}

	localCanceled := s.registry.CancelVideos(ids)

	if s.broadcaster == nil || !s.broadcaster.Available() {
		logctx.With(ctx, s.logger).Info("transcode tasks canceled locally", zap.String("event", "media.cancel_service.notify_video_deleted.diagnostic"), zap.String("operation", "media.cancel_service.notify_video_deleted"),
			zap.Int("tasks", localCanceled),
			zap.Ints("video_ids", uintsToInts(ids)),
		)

		return nil
	}

	if err := s.broadcaster.PublishVideoDeleted(ctx, ids); err != nil {
		return err
	}

	logctx.With(ctx, s.logger).Info("transcode cancellation broadcast", zap.String("event", "media.cancel_service.notify_video_deleted.diagnostic"), zap.String("operation", "media.cancel_service.notify_video_deleted"),
		zap.Int("local_tasks", localCanceled),
		zap.Ints("video_ids", uintsToInts(ids)),
	)

	return nil
}

// uintsToInts 把视频 ID 列表转成 zap.Ints 可用的切片（与原实现同名辅助函数一致）。
func uintsToInts(values []uint) []int {
	out := make([]int, 0, len(values))
	for _, value := range values {
		out = append(out, int(value))
	}

	return out
}
