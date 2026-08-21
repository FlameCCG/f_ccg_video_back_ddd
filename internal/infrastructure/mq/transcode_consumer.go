package mq

import (
	"context"
	"errors"
	"fmt"
	"strings"

	mediaapp "f_ccg_video_back_ddd/internal/application/media"
	media "f_ccg_video_back_ddd/internal/domain/media"
	"f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// -----------------------------------------------------------------------------
// Kafka 转码消费者：使用 workerPool 控制并发，避免打满 CPU/IO。
// -----------------------------------------------------------------------------

// CancelSubscriberPort 取消事件订阅端口（由 cache/media.CancelSubscriber 实现）。
//
// 声明在此处而非应用层：订阅生命周期与 Kafka 消费者绑定
// （原实现同样在 TranscodeConsumer.Start/Stop 内启停）。
type CancelSubscriberPort interface {
	// Start 在返回前完成订阅，避免消费者启动与监听就绪之间丢事件。
	Start(ctx context.Context) error
	// Stop 结束订阅并等待其 goroutine 退出。
	Stop() error
}

// TranscodeConsumer 消费转码任务并以有限 worker 池并发处理。
type TranscodeConsumer struct {
	consumer   *KafkaConsumer
	workerPool chan struct{} // 并发控制池
	service    *mediaapp.TranscodeService
	registry   *media.TaskRegistry
	subscriber CancelSubscriberPort
	logger     *zap.Logger
}

// NewTranscodeConsumer 创建转码 Kafka 消费者。
//
// 错误文案与顺序与原实现完全一致（消费者初始化重试的错误分类依赖这些文案）。
func NewTranscodeConsumer(
	cfg *config.Kafka,
	service *mediaapp.TranscodeService,
	registry *media.TaskRegistry,
	subscriber CancelSubscriberPort,
	producers *ProducerPool,
	logger *zap.Logger,
) (*TranscodeConsumer, error) {
	if service == nil {
		return nil, errors.New("application config not initialized")
	}

	if cfg == nil || !cfg.Enable {
		return nil, errors.New("kafka not enabled")
	}

	if !service.TranscodeEnabled() {
		return nil, errors.New("transcode config not configured")
	}

	topic := strings.TrimSpace(cfg.Topics[TopicTranscode])
	if topic == "" {
		return nil, errors.New("transcode topic not configured")
	}

	tc := &TranscodeConsumer{
		workerPool: make(chan struct{}, service.MaxWorkers()),
		service:    service,
		registry:   registry,
		subscriber: subscriber,
		logger:     logger,
	}

	consumer, err := NewKafkaConsumer(cfg, []string{topic}, tc.handleMessage, producers, logger)
	if err != nil {
		return nil, err
	}

	tc.consumer = consumer

	return tc, nil
}

// Start 启动转码消费者：先订阅取消事件，再开消费循环，最后拉起断点目录清扫。
func (tc *TranscodeConsumer) Start() error {
	if tc.consumer == nil {
		return errors.New("consumer not initialized")
	}

	if tc.subscriber != nil {
		if err := tc.subscriber.Start(tc.consumer.ShutdownContext()); err != nil {
			return err
		}
	}

	if err := tc.consumer.Start(); err != nil {
		if tc.subscriber != nil {
			_ = tc.subscriber.Stop()
		}

		return err
	}

	// 断点目录孤儿清扫：清理任务不再重试后遗留的编码产物（如 DLQ 前的历史断点）。
	tc.service.StartCheckpointJanitor(tc.consumer.ShutdownContext())

	return nil
}

// Stop 停止转码消费者：先停消费循环，再退订取消事件，最后释放 consumer group。
func (tc *TranscodeConsumer) Stop() error {
	if tc.consumer != nil {
		tc.consumer.StopConsuming()
	}

	if tc.subscriber != nil {
		if err := tc.subscriber.Stop(); err != nil {
			tc.logger.Warn("stop transcode cancel subscriber failed", zap.Error(err))
		}
	}

	if tc.consumer != nil {
		return tc.consumer.Close()
	}

	return nil
}

// handleMessage 处理一条 Kafka 消息：登记取消上下文 → 抢 worker → 跑转码 → 决定是否提交 offset。
func (tc *TranscodeConsumer) handleMessage(session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) error {
	key := taskIdentityFromMessage(msg.Value)

	tc.logger.Info("transcode task received",
		zap.Uint("videoId", key.VideoID),
		zap.Uint("partId", key.PartID),
		zap.String("topic", msg.Topic),
		zap.Int32("partition", msg.Partition),
		zap.Int64("offset", msg.Offset),
		zap.String("memberId", session.MemberID()),
		zap.Int32("generation", session.GenerationID()),
		zap.String("key", string(msg.Key)),
	)
	tc.logger.Debug("transcode task payload",
		zap.Uint("videoId", key.VideoID),
		zap.Int64("offset", msg.Offset),
		zap.String("value", string(msg.Value)),
	)

	baseCtx, releaseBaseCtx := tc.taskContext(session.Context())
	defer releaseBaseCtx()

	taskCtx, unregister := tc.registry.Register(baseCtx, key)
	defer unregister()

	if err := tc.acquireWorker(taskCtx); err != nil {
		return tc.finishMessage(session, msg, taskCtx, nil, fmt.Errorf("wait transcode worker failed: %w", err))
	}

	defer func() { <-tc.workerPool }()

	pt := tc.service.NewProgressTracker(key.VideoID, key.PartID)
	taskCtx = media.ContextWithProgressTracker(taskCtx, pt)

	return tc.finishMessage(session, msg, taskCtx, pt, tc.processMessage(taskCtx, msg.Value))
}

// processMessage 反序列化消息并交由应用层执行（原 processMessage 的入口部分）。
func (tc *TranscodeConsumer) processMessage(ctx context.Context, payloadBytes []byte) error {
	payload, err := decodeTranscodePayload(payloadBytes)
	if err != nil {
		return media.PermanentMessage(fmt.Errorf("unmarshal payload failed: %w", err))
	}

	return tc.service.ProcessTask(ctx, payload.Task())
}

// finishMessage 依据处理结果决定是否提交 offset（原 finishMessage）。
func (tc *TranscodeConsumer) finishMessage(
	session sarama.ConsumerGroupSession,
	msg *sarama.ConsumerMessage,
	taskCtx context.Context,
	pt *media.TranscodeProgressTracker,
	processErr error,
) error {
	key := taskIdentityFromMessage(msg.Value)

	if media.TaskWasDeleted(taskCtx, processErr) {
		tc.service.ClearProgress(taskCtx, key.VideoID, key.PartID)
		session.MarkMessage(msg, "video deleted; transcode canceled")
		tc.logger.Info("deleted video transcode task skipped",
			zap.Uint("videoId", key.VideoID),
			zap.Uint("partId", key.PartID),
			zap.String("topic", msg.Topic),
			zap.Int32("partition", msg.Partition),
			zap.Int64("offset", msg.Offset),
		)

		return nil
	}

	if processErr != nil {
		tc.logTranscodeSkipCommit(session, msg, key, taskCtx, pt, processErr, "process_error")

		return processErr
	}

	if cause := context.Cause(taskCtx); cause != nil {
		tc.logTranscodeSkipCommit(session, msg, key, taskCtx, pt, cause, "task_context_cause")

		return cause
	}

	session.MarkMessage(msg, "")
	tc.logger.Info("transcode kafka offset marked",
		zap.Uint("videoId", key.VideoID),
		zap.Uint("partId", key.PartID),
		zap.String("topic", msg.Topic),
		zap.Int32("partition", msg.Partition),
		zap.Int64("offset", msg.Offset),
		zap.String("memberId", session.MemberID()),
		zap.Int32("generation", session.GenerationID()),
	)

	return nil
}

// logTranscodeSkipCommit 记录在途任务将被重投递的原因（原 logTranscodeSkipCommit）。
func (tc *TranscodeConsumer) logTranscodeSkipCommit(
	session sarama.ConsumerGroupSession,
	msg *sarama.ConsumerMessage,
	key media.TaskKey,
	taskCtx context.Context,
	pt *media.TranscodeProgressTracker,
	err error,
	source string,
) {
	percent, stage, status, outTimeSec := pt.DiagnosticSnapshot()
	reason := media.ClassifyInterruptReason(taskCtx, err, session.Context().Err() != nil)

	tc.logger.Warn("transcode unfinished, skip kafka offset commit (will redeliver; checkpoint resumes finished encodes)",
		zap.String("source", source),
		zap.String("reason", reason),
		zap.Uint("videoId", key.VideoID),
		zap.Uint("partId", key.PartID),
		zap.String("topic", msg.Topic),
		zap.Int32("partition", msg.Partition),
		zap.Int64("offset", msg.Offset),
		zap.String("memberId", session.MemberID()),
		zap.Int32("generation", session.GenerationID()),
		zap.String("stage", stage),
		zap.String("status", status),
		zap.Float64("percent", percent),
		zap.Float64("outTimeSec", outTimeSec),
		zap.NamedError("err", err),
		zap.NamedError("taskCause", context.Cause(taskCtx)),
		zap.NamedError("sessionErr", session.Context().Err()),
		zap.NamedError("sessionCause", context.Cause(session.Context())),
	)
}

// taskContext 把 Kafka 会话所有权与进程停止联动（原 taskContext）。
//
// rebalance 必须取消旧持有者，避免同一分区被重新分配后同一任务跑两次。
func (tc *TranscodeConsumer) taskContext(sessionCtx context.Context) (context.Context, context.CancelFunc) {
	if sessionCtx == nil {
		sessionCtx = context.Background()
	}

	ctx, cancel := context.WithCancel(sessionCtx)
	if tc.consumer == nil {
		return ctx, cancel
	}

	stopShutdownLink := context.AfterFunc(tc.consumer.ShutdownContext(), cancel)
	release := func() {
		stopShutdownLink()
		cancel()
	}

	return ctx, release
}

// acquireWorker 抢占一个 worker 名额；ctx 取消时放弃。
func (tc *TranscodeConsumer) acquireWorker(ctx context.Context) error {
	select {
	case tc.workerPool <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
