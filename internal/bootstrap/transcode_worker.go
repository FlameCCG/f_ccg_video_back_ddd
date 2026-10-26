package bootstrap

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/infrastructure/mq"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"fmt"

	"go.uber.org/zap"
)

type transcodeConsumer interface {
	Start() error
	Stop() error
}

// TranscodeWorker 只管理转码消费及其附属资源，不启动其他后台组件。
type TranscodeWorker struct {
	consumer  transcodeConsumer
	producers *mq.ProducerPool
	detached  *ctxutil.DetachedTaskGroup
	logger    *zap.Logger
}

// NewTranscodeWorker 装配独立转码进程。直接使用消费者，不受 HTTP 进程的
// transcode.localConsumerDisabled 开关影响。
func NewTranscodeWorker(
	consumer *mq.TranscodeConsumer,
	producers *mq.ProducerPool,
	detached *ctxutil.DetachedTaskGroup,
	logger *zap.Logger,
) *TranscodeWorker {
	return &TranscodeWorker{consumer: consumer, producers: producers, detached: detached, logger: logger}
}

// Run 消费至退出信号，并在基础设施被清理前等待在途转码结束、回收资源。
func (w *TranscodeWorker) Run(ctx context.Context) (err error) {
	defer func() {
		err = errors.Join(err, w.consumer.Stop())
		err = errors.Join(err, w.producers.Close())

		cleanupCtx, cancel := context.WithTimeout(context.Background(), ctxutil.AsyncLongTimeout)
		defer cancel()

		err = errors.Join(err, w.detached.Stop(cleanupCtx))
	}()

	if ctx.Err() != nil {
		return nil
	}

	if err := w.consumer.Start(); err != nil {
		return fmt.Errorf("start transcode consumer: %w", err)
	}

	w.logger.Info("process.started", zap.String("event", "process.started"), zap.String("component", "transcode_worker"))
	<-ctx.Done()
	w.logger.Info("process.stopping", zap.String("event", "process.stopping"), zap.String("component", "transcode_worker"))

	return nil
}

// ReportError reports a worker failure to the process supervisor.
func (w *TranscodeWorker) ReportError(err error) {
	w.logger.Error("process.failed", zap.String("event", "process.failed"), zap.String("component", "transcode_worker"), zap.Error(err))
}
