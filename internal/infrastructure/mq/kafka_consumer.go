package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"f_ccg_video_back_ddd/internal/config"
	media "f_ccg_video_back_ddd/internal/domain/media"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// MessageHandler 业务消费回调，需自行确认 offset。
//
// 回调负责在可确认分支调用 session.MarkMessage；标记会进入 Sarama 的 offset 提交流程，
// 不等于 broker 此刻已持久提交，也不排除崩溃后的重复消费。
type MessageHandler func(session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) error

// KafkaConsumer 包装 sarama.ConsumerGroup 的生命周期。
type KafkaConsumer struct {
	cfg           *config.Kafka
	group         sarama.ConsumerGroup
	groupMu       sync.RWMutex
	closed        bool
	topics        []string
	fn            MessageHandler
	producers     *ProducerPool
	dlqProducer   sarama.SyncProducer
	ownsProducers bool
	logger        *zap.Logger
	// consumeCtx 仅控制拉取循环；与长任务处理解耦，避免 rebalance 取消转码。
	consumeCtx    context.Context
	consumeCancel context.CancelFunc
	// shutdownCtx 供转码等长任务使用；进程关闭时先停消费、等待在途任务后再取消。
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	wg             sync.WaitGroup
}

const (
	consumerRetryBackoff    = 3 * time.Second
	messageRetryBackoff     = 500 * time.Millisecond
	maxMessageRetryAttempts = 3
)

const (
	// defaultKafkaSessionTimeout 对应 broker 侧 session.timeout.ms：
	// 消费者多久没收到心跳就被认为下线并触发 rebalance。必须大于心跳间隔。
	defaultKafkaSessionTimeout = 45 * time.Second
	// defaultKafkaHeartbeat 对应 heartbeat.interval.ms：
	// 消费者向 group coordinator 发送心跳的周期，维持成员存活状态。
	defaultKafkaHeartbeat = 10 * time.Second
	// defaultKafkaRebalanceTimeout 对应 rebalance.timeout.ms：
	// rebalance 时等待各成员释放旧 partition 分配的最长时间；转码同步消费较慢，默认给足 4 小时。
	defaultKafkaRebalanceTimeout = 4 * time.Hour
	// defaultKafkaNetTimeout 对应各次网络 I/O（建连/读/写）的单次超时上限。
	defaultKafkaNetTimeout = 30 * time.Second
	// defaultKafkaMetadataBackoff 拉取集群元数据（broker/topic/partition 信息）失败后的重试间隔。
	defaultKafkaMetadataBackoff = 2 * time.Second
)

// NewKafkaConsumer 构建独立的 consumer group，避免共享实例在重平衡/重建时互相影响。
func NewKafkaConsumer(
	cfg *config.Kafka,
	topics []string,
	handler MessageHandler,
	producers *ProducerPool,
	logger *zap.Logger,
) (*KafkaConsumer, error) {
	if cfg == nil || len(cfg.Brokers) == 0 {
		return nil, errors.New("kafka brokers not configured")
	}

	if strings.TrimSpace(cfg.GroupID) == "" {
		return nil, errors.New("kafka groupID required")
	}

	if len(topics) == 0 {
		return nil, errors.New("topics list required")
	}

	if handler == nil {
		return nil, errors.New("message handler required")
	}

	if err := validateConsumerDLQ(cfg, topics, logger); err != nil {
		return nil, err
	}
	ownsProducers := producers == nil
	if ownsProducers {
		producers = NewProducerPool(logger)
	}
	dlqProducer, err := producers.Producer(cfg)
	if err != nil {
		return nil, fmt.Errorf("initialize dlq producer: %w", err)
	}
	cg, err := newConsumerGroup(cfg, logger)
	if err != nil {
		if ownsProducers {
			_ = producers.Close()
		}
		return nil, err
	}

	consumeCtx, consumeCancel := context.WithCancel(context.Background())
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	return &KafkaConsumer{
		cfg:            cfg,
		group:          cg,
		topics:         topics,
		fn:             handler,
		producers:      producers,
		dlqProducer:    dlqProducer,
		ownsProducers:  ownsProducers,
		logger:         logger,
		consumeCtx:     consumeCtx,
		consumeCancel:  consumeCancel,
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
	}, nil
}

// validateConsumerDLQ fails before consuming if deployment omitted the dead-letter topic.
// Topic creation is explicit in docker-compose; production operators may provision it with their own replication policy.
func validateConsumerDLQ(cfg *config.Kafka, topics []string, logger *zap.Logger) error {
	topic := resolveDLQTopic(cfg, topics)
	admin, err := sarama.NewClusterAdmin(cfg.Brokers, newConsumerGroupConfig(cfg, logger))
	if err != nil {
		return fmt.Errorf("initialize dlq metadata: %w", err)
	}
	defer admin.Close()
	metadata, err := admin.DescribeTopics([]string{topic})
	if err != nil {
		return fmt.Errorf("describe dlq topic %q: %w", topic, err)
	}
	if len(metadata) != 1 || metadata[0].Err != sarama.ErrNoError {
		return fmt.Errorf("dlq topic %q must be created before starting the consumer", topic)
	}
	return nil
}

func newConsumerGroup(cfg *config.Kafka, logger *zap.Logger) (sarama.ConsumerGroup, error) {
	kCfg := newConsumerGroupConfig(cfg, logger)

	return sarama.NewConsumerGroup(cfg.Brokers, cfg.GroupID, kCfg)
}

// newConsumerGroupConfig 构建 Sarama Consumer Group 客户端配置。
// 下列每一项都对应 Kafka 消费组协议中的具体行为，影响连接稳定性、rebalance 与 offset 语义。
func newConsumerGroupConfig(cfg *config.Kafka, logger *zap.Logger) *sarama.Config {
	// 创建带 Sarama 默认值的基础配置（例如 AutoCommit.Enable=true、AutoCommit.Interval=1s 等）。
	kCfg := sarama.NewConfig()

	// ClientID：客户端逻辑标识，会出现在 broker 日志与监控里，便于区分不同服务实例。
	kCfg.ClientID = cfg.ClientID

	// Net.DialTimeout：与 broker 建立 TCP 连接的最长等待时间，超时视为网络不可达。
	kCfg.Net.DialTimeout = defaultKafkaNetTimeout
	// Net.ReadTimeout：单次读 broker 响应的最长等待时间，防止读挂死。
	kCfg.Net.ReadTimeout = defaultKafkaNetTimeout
	// Net.WriteTimeout：单次向 broker 写请求的最长等待时间，防止写挂死。
	kCfg.Net.WriteTimeout = defaultKafkaNetTimeout

	// Metadata.Retry.Max：刷新集群元数据失败时的最大重试次数（例如 broker 短暂不可用）。
	kCfg.Metadata.Retry.Max = 6
	// Metadata.Retry.Backoff：元数据重试之间的退避间隔，避免频繁打 broker。
	kCfg.Metadata.Retry.Backoff = defaultKafkaMetadataBackoff

	// Consumer.Group.Rebalance.Strategy：分区分配策略。
	// Range：按 topic 把连续 partition 段分给各消费者；多 topic/多实例时常见，便于顺序消费同 topic 邻近分区。
	kCfg.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRange()

	// Consumer.Group.Session.Timeout：会话超时（session.timeout.ms）。
	// 若消费者在该时间内未被 coordinator 判定为存活，会被踢出组并触发 rebalance。
	kCfg.Consumer.Group.Session.Timeout = defaultKafkaSessionTimeout

	// Consumer.Group.Heartbeat.Interval：心跳周期（heartbeat.interval.ms）。
	// 消费者按该间隔上报“我还活着”；通常设为 Session.Timeout 的 1/3 左右。
	kCfg.Consumer.Group.Heartbeat.Interval = defaultKafkaHeartbeat

	// Consumer.Group.Rebalance.Timeout：rebalance 超时（rebalance.timeout.ms）。
	// 协调分区再分配时，等待所有成员交还旧 claim 的上限；同步长任务消费需足够大，否则协调可能失败。
	kCfg.Consumer.Group.Rebalance.Timeout = resolveKafkaRebalanceTimeout(cfg)

	// Consumer.Offsets.Initial：组内没有任何已提交 offset 时的起始位置。
	// OffsetOldest：从最早消息开始，避免新组/丢 offset 时跳过积压转码任务。
	kCfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	// Consumer.Return.Errors：为 true 时异步消费错误会写入 Errors() 通道，便于监控与告警（本项目未单独消费该通道）。
	kCfg.Consumer.Return.Errors = true

	// Version：显式声明 Kafka 协议版本，使 Sarama 按对应版本序列化请求/解析响应；不配则用库默认版本。
	if v := strings.TrimSpace(cfg.Version); v != "" {
		version, err := sarama.ParseKafkaVersion(v)
		if err != nil {
			logger.Warn("parse kafka version failed", zap.Error(err))
		} else {
			kCfg.Version = version
		}
	}

	return kCfg
}

// Start 启动消费循环。
func (c *KafkaConsumer) Start() error {
	if c.currentGroup() == nil {
		return errors.New("consumer group not ready")
	}
	// 启动消费循环
	c.wg.Add(1)

	go c.loop()

	c.logger.Info("kafka consumer started", zap.Strings("topics", c.topics), zap.String("group", c.cfg.GroupID))

	return nil
}

// ShutdownContext 返回长任务（如转码）应使用的上下文，不受 Kafka rebalance/session 结束影响。
func (c *KafkaConsumer) ShutdownContext() context.Context {
	if c == nil || c.shutdownCtx == nil {
		return context.Background()
	}

	return c.shutdownCtx
}

// StopConsuming 停止消费循环，但不取消 ShutdownContext，便于在途长任务自然结束。
func (c *KafkaConsumer) StopConsuming() {
	if c == nil {
		return
	}

	if c.consumeCancel != nil {
		c.consumeCancel()
	}

	c.wg.Wait()
}

// Close 取消长任务上下文并关闭 consumer group。
func (c *KafkaConsumer) Close() error {
	if c == nil {
		return nil
	}

	if c.shutdownCancel != nil {
		c.shutdownCancel()
	}

	c.groupMu.Lock()
	c.closed = true
	group := c.group
	c.group = nil
	c.groupMu.Unlock()
	var closeErr error
	if group != nil {
		closeErr = group.Close()
	}
	if c.ownsProducers {
		closeErr = errors.Join(closeErr, c.producers.Close())
	}
	return closeErr
}

// Stop 完整关闭：先停消费，再取消长任务并释放 consumer group。
func (c *KafkaConsumer) Stop() error {
	c.StopConsuming()

	return c.Close()
}

func (c *KafkaConsumer) loop() {
	defer c.wg.Done()

	handler := consumerGroupHandler{
		shutdownCtx: c.shutdownCtx,
		fn:          c.fn,
		producer:    c.dlqProducer,
		dlqTopic:    resolveDLQTopic(c.cfg, c.topics),
		logger:      c.logger,
	}

	for {
		if c.consumeCtx.Err() != nil {
			return
		}

		group := c.currentGroup()
		if group == nil {
			return
		}
		if err := group.Consume(c.consumeCtx, c.topics, handler); err != nil {
			if errors.Is(err, sarama.ErrClosedConsumerGroup) {
				if c.consumeCtx.Err() != nil {
					return
				}

				if recreateErr := c.recreateGroup(); recreateErr != nil {
					c.logger.Error("recreate kafka consumer group failed", zap.Error(recreateErr))
					time.Sleep(consumerRetryBackoff)
				}

				continue
			}

			if shouldRecreateConsumerGroup(err) {
				c.logger.Warn("kafka consumer group unhealthy, recreating", zap.Error(err))

				if recreateErr := c.recreateGroup(); recreateErr != nil {
					c.logger.Error("recreate kafka consumer group failed", zap.Error(recreateErr))
				}
			} else {
				c.logger.Error("kafka consume failed", zap.Error(err))
			}

			if c.consumeCtx.Err() != nil {
				return
			}

			time.Sleep(consumerRetryBackoff)
		}
	}
}

func resolveKafkaRebalanceTimeout(cfg *config.Kafka) time.Duration {
	if cfg != nil && cfg.RebalanceTimeoutSec > 0 {
		return time.Duration(cfg.RebalanceTimeoutSec) * time.Second
	}

	return defaultKafkaRebalanceTimeout
}

func (c *KafkaConsumer) currentGroup() sarama.ConsumerGroup {
	c.groupMu.RLock()
	defer c.groupMu.RUnlock()
	return c.group
}

func (c *KafkaConsumer) recreateGroup() error {
	nextGroup, err := newConsumerGroup(c.cfg, c.logger)
	if err != nil {
		return err
	}

	c.groupMu.Lock()
	if c.closed {
		c.groupMu.Unlock()
		_ = nextGroup.Close()
		return sarama.ErrClosedConsumerGroup
	}
	oldGroup := c.group
	c.group = nextGroup
	c.groupMu.Unlock()

	if oldGroup != nil {
		_ = oldGroup.Close()
	}

	return nil
}

func shouldRecreateConsumerGroup(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, sarama.ErrOutOfBrokers) {
		return true
	}

	if isKafkaNetworkTimeout(err) {
		return true
	}

	var kafkaErr sarama.KError
	if errors.As(err, &kafkaErr) {
		switch kafkaErr {
		case sarama.ErrConsumerCoordinatorNotAvailable,
			sarama.ErrNotCoordinatorForConsumer,
			sarama.ErrIllegalGeneration,
			sarama.ErrUnknownMemberId,
			sarama.ErrRebalanceInProgress,
			sarama.ErrOffsetsLoadInProgress:
			return true
		}
	}

	message := strings.ToLower(err.Error())

	return strings.Contains(message, "not coordinated by this broker") ||
		strings.Contains(message, "i/o timeout") ||
		strings.Contains(message, "coordinator") ||
		strings.Contains(message, "unknown member id") ||
		strings.Contains(message, "illegal generation") ||
		strings.Contains(message, "rebalance in progress")
}

func isKafkaNetworkTimeout(err error) bool {
	if err == nil {
		return false
	}

	var netErr net.Error

	return errors.As(err, &netErr) && netErr.Timeout()
}

type consumerGroupHandler struct {
	shutdownCtx context.Context
	fn          MessageHandler
	producer    sarama.SyncProducer
	dlqTopic    string
	logger      *zap.Logger
}

func (h consumerGroupHandler) log() *zap.Logger {
	if h.logger == nil {
		return zap.NewNop()
	}

	return h.logger
}

// Setup 在每轮 rebalance 分配完成后记录成员与 claim 信息。
func (h consumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	h.log().Info("kafka consumer rebalance setup",
		zap.String("memberId", session.MemberID()),
		zap.Int32("generation", session.GenerationID()),
		zap.Any("claims", session.Claims()),
	)

	return nil
}

// Cleanup 在会话结束时记录退出原因，便于排查 rebalance 与关机导致的重投递。
func (h consumerGroupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	h.log().Info("kafka consumer rebalance cleanup",
		zap.String("memberId", session.MemberID()),
		zap.Int32("generation", session.GenerationID()),
		zap.Any("claims", session.Claims()),
		zap.NamedError("sessionErr", session.Context().Err()),
		zap.NamedError("sessionCause", context.Cause(session.Context())),
	)

	return nil
}

// ConsumeClaim 消费单个分区的消息流。
func (h consumerGroupHandler) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	h.log().Info("kafka claim started",
		zap.String("topic", claim.Topic()),
		zap.Int32("partition", claim.Partition()),
		zap.Int64("initialOffset", claim.InitialOffset()),
		zap.String("memberId", session.MemberID()),
		zap.Int32("generation", session.GenerationID()),
	)

	for {
		select {
		case msg := <-claim.Messages():
			if msg == nil {
				h.log().Info("kafka claim messages channel closed",
					zap.String("topic", claim.Topic()),
					zap.Int32("partition", claim.Partition()),
					zap.String("memberId", session.MemberID()),
					zap.Int32("generation", session.GenerationID()),
				)

				return nil
			}

			if err := h.handleMessage(session, msg); err != nil {
				// 会话/上下文已取消时结束 claim，避免把取消错误当业务失败反复重启
				if h.isStopping(session) {
					h.log().Warn("kafka claim ending due to session/context cancel (offset not committed; may redeliver)",
						zap.String("topic", msg.Topic),
						zap.Int32("partition", msg.Partition),
						zap.Int64("offset", msg.Offset),
						zap.String("memberId", session.MemberID()),
						zap.Int32("generation", session.GenerationID()),
						zap.NamedError("handlerErr", err),
						zap.NamedError("sessionErr", session.Context().Err()),
						zap.NamedError("sessionCause", context.Cause(session.Context())),
					)

					return nil //nolint:nilerr // claim ends with session/context cancellation
				}

				var permanentErr *media.PermanentMessageError
				if errors.As(err, &permanentErr) {
					if dlqErr := h.publishDLQ(msg, permanentErr.Unwrap()); dlqErr != nil {
						return fmt.Errorf("publish dlq message: %w", dlqErr)
					}

					session.MarkMessage(msg, "permanent failure sent to dlq")
					h.log().Error("permanent kafka message sent to dlq",
						zap.Error(permanentErr.Unwrap()),
						zap.String("topic", msg.Topic),
						zap.Int32("partition", msg.Partition),
						zap.Int64("offset", msg.Offset),
						zap.String("dlqTopic", h.dlqTopic))

					continue
				}

				h.log().Error("handle kafka message failed",
					zap.Error(err),
					zap.String("topic", msg.Topic),
					zap.Int32("partition", msg.Partition),
					zap.Int64("offset", msg.Offset),
					zap.String("memberId", session.MemberID()),
					zap.Int32("generation", session.GenerationID()),
				)

				return err
			}
		case <-session.Context().Done():
			h.log().Warn("kafka claim session done (rebalance/shutdown; in-flight offset may redeliver)",
				zap.String("topic", claim.Topic()),
				zap.Int32("partition", claim.Partition()),
				zap.String("memberId", session.MemberID()),
				zap.Int32("generation", session.GenerationID()),
				zap.NamedError("sessionErr", session.Context().Err()),
				zap.NamedError("sessionCause", context.Cause(session.Context())),
			)

			return nil
		}
	}
}

func (h consumerGroupHandler) isStopping(session sarama.ConsumerGroupSession) bool {
	return session.Context().Err() != nil || (h.shutdownCtx != nil && h.shutdownCtx.Err() != nil)
}

func (h consumerGroupHandler) handleMessage(
	session sarama.ConsumerGroupSession,
	msg *sarama.ConsumerMessage,
) error {
	var lastErr error

	for attempt := 0; attempt <= maxMessageRetryAttempts; attempt++ {
		if attempt > 0 {
			h.log().Warn("kafka message handler retry",
				zap.Int("attempt", attempt),
				zap.Int("maxAttempts", maxMessageRetryAttempts),
				zap.String("topic", msg.Topic),
				zap.Int32("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.NamedError("lastErr", lastErr),
			)
		}

		err := h.fn(session, msg)
		if err == nil {
			return nil
		}

		if h.isStopping(session) {
			h.log().Warn("kafka message handler stopped by session/context cancel",
				zap.Int("attempt", attempt),
				zap.String("topic", msg.Topic),
				zap.Int32("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.NamedError("handlerErr", err),
				zap.NamedError("sessionErr", session.Context().Err()),
				zap.NamedError("sessionCause", context.Cause(session.Context())),
			)

			return err
		}

		var permanentErr *media.PermanentMessageError
		if errors.As(err, &permanentErr) {
			return permanentErr
		}

		lastErr = err

		if attempt == maxMessageRetryAttempts {
			h.log().Error("kafka message handler exhausted retries",
				zap.Int("attempts", attempt+1),
				zap.String("topic", msg.Topic),
				zap.Int32("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.Error(lastErr),
			)

			break
		}

		timer := time.NewTimer(messageRetryBackoff * time.Duration(attempt+1))
		select {
		case <-timer.C:
		case <-session.Context().Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}

			h.log().Warn("kafka message handler retry aborted by session cancel",
				zap.Int("attempt", attempt),
				zap.String("topic", msg.Topic),
				zap.Int32("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.NamedError("lastErr", lastErr),
				zap.NamedError("sessionErr", session.Context().Err()),
				zap.NamedError("sessionCause", context.Cause(session.Context())),
			)

			return session.Context().Err()
		}
	}

	return media.PermanentMessage(fmt.Errorf("retries exhausted: %w", lastErr))
}

// dlqMessage 是死信队列消息体（外部契约）。
type dlqMessage struct {
	Topic     string    `json:"topic"`
	Partition int32     `json:"partition"`
	Offset    int64     `json:"offset"`
	Key       string    `json:"key,omitempty"`
	Value     string    `json:"value"`
	Error     string    `json:"error"`
	FailedAt  time.Time `json:"failedAt"`
}

func (h consumerGroupHandler) publishDLQ(msg *sarama.ConsumerMessage, cause error) error {
	if strings.TrimSpace(h.dlqTopic) == "" {
		return errors.New("dlq topic not configured")
	}

	if h.producer == nil {
		return errors.New("dlq producer not initialized")
	}

	payload := dlqMessage{
		Topic:     msg.Topic,
		Partition: msg.Partition,
		Offset:    msg.Offset,
		Key:       string(msg.Key),
		Value:     string(msg.Value),
		Error:     cause.Error(),
		FailedAt:  time.Now().UTC(),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	producerMsg := &sarama.ProducerMessage{
		Topic: h.dlqTopic,
		Key:   sarama.ByteEncoder(msg.Key),
		Value: sarama.ByteEncoder(raw),
	}
	_, _, err = h.producer.SendMessage(producerMsg)

	return err
}

func resolveDLQTopic(cfg *config.Kafka, topics []string) string {
	if cfg != nil {
		for _, key := range []string{"dlq", "transcode_dlq"} {
			if topic := strings.TrimSpace(cfg.Topics[key]); topic != "" {
				return topic
			}
		}
	}

	if len(topics) > 0 && strings.TrimSpace(topics[0]) != "" {
		return strings.TrimSpace(topics[0]) + ".dlq"
	}

	return ""
}
