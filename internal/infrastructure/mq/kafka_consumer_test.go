package mq

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/config"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	media "f_ccg_video_back_ddd/internal/domain/media"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

func TestShouldRecreateConsumerGroup(t *testing.T) {
	t.Parallel()

	t.Run("nil error", func(t *testing.T) {
		t.Parallel()

		require.False(t, shouldRecreateConsumerGroup(nil), "nil error should not recreate consumer group")
	})

	t.Run("kafka coordinator error", func(t *testing.T) {
		t.Parallel()

		require.True(t, shouldRecreateConsumerGroup(sarama.ErrNotCoordinatorForConsumer), "ErrNotCoordinatorForConsumer should recreate consumer group")
	})

	t.Run("wrapped broker exhaustion error", func(t *testing.T) {
		t.Parallel()

		err := sarama.Wrap(sarama.ErrOutOfBrokers, errors.New("dial tcp timeout"))
		require.True(t, shouldRecreateConsumerGroup(err), "wrapped ErrOutOfBrokers should recreate consumer group")
	})

	t.Run("plain string match", func(t *testing.T) {
		t.Parallel()

		err := errors.New("kafka server: Request was for a consumer group that is not coordinated by this broker")
		require.True(t, shouldRecreateConsumerGroup(err), "coordinator string error should recreate consumer group")
	})

	t.Run("network timeout", func(t *testing.T) {
		t.Parallel()

		err := &net.OpError{Op: "read", Net: "tcp", Err: timeoutError{}}
		require.True(t, shouldRecreateConsumerGroup(err), "network timeout should recreate consumer group")
	})

	t.Run("unrelated error", func(t *testing.T) {
		t.Parallel()

		require.False(t, shouldRecreateConsumerGroup(errors.New("boom")), "unrelated error should not recreate consumer group")
	})
}

func TestNewConsumerGroupConfigUsesOldestOffset(t *testing.T) {
	t.Parallel()

	cfg := &config.Kafka{
		ClientID: "test-client",
		Version:  "3.9.0",
	}

	kCfg := newConsumerGroupConfig(cfg, zap.NewNop())
	require.Equal(t, sarama.OffsetOldest, kCfg.Consumer.Offsets.Initial, "expected initial offset %d, got %d", sarama.OffsetOldest, kCfg.Consumer.Offsets.Initial)
}

func TestResolveKafkaRebalanceTimeout(t *testing.T) {
	t.Parallel()

	{
		got := resolveKafkaRebalanceTimeout(nil)
		require.Equal(t, defaultKafkaRebalanceTimeout, got, "expected default rebalance timeout %v, got %v", defaultKafkaRebalanceTimeout, got)
	}

	cfg := &config.Kafka{RebalanceTimeoutSec: 600}
	{
		got := resolveKafkaRebalanceTimeout(cfg)
		require.Equal(t, 600*time.Second, got, "expected configured rebalance timeout 600s, got %v", got)
	}
}

func TestStopConsumingDoesNotCancelShutdownContext(t *testing.T) {
	t.Parallel()

	consumeCtx, consumeCancel := context.WithCancel(context.Background())

	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	defer shutdownCancel()

	consumer := &KafkaConsumer{
		consumeCtx:     consumeCtx,
		consumeCancel:  consumeCancel,
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
	}

	consumer.StopConsuming()

	{
		err := consumer.consumeCtx.Err()
		require.ErrorIs(t, err, context.Canceled, "expected consume ctx canceled, got %v", err)
	}

	{
		err := consumer.ShutdownContext().Err()
		require.NoError(t, err, "shutdown ctx should remain active after StopConsuming, got %v", err)
	}
}

func TestCloseCancelsShutdownContext(t *testing.T) {
	t.Parallel()

	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	defer shutdownCancel()

	consumer := &KafkaConsumer{
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
	}

	{
		err := consumer.Close()
		require.NoError(t, err, "close consumer failed: %v", err)
	}

	require.ErrorIs(t, consumer.ShutdownContext().Err(), context.Canceled, "expected shutdown ctx canceled after Close, got %v", consumer.ShutdownContext().Err())
}

func TestConsumeClaimSuppressesContextCanceledErrors(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler := consumerGroupHandler{
		fn: func(_ sarama.ConsumerGroupSession, _ *sarama.ConsumerMessage) error {
			return context.Canceled
		},
	}

	err := handler.ConsumeClaim(stubConsumerGroupSession{ctx: ctx}, stubConsumerGroupClaim{
		messages: make(chan *sarama.ConsumerMessage, 1),
	})
	require.NoError(t, err, "expected canceled session error to be suppressed, got %v", err)
}

// TestNewKafkaConsumerValidatesInput 构造校验的错误文案属于消费者重试分类的输入，必须逐字保持。
func TestNewKafkaConsumerValidatesInput(t *testing.T) {
	t.Parallel()

	handler := func(sarama.ConsumerGroupSession, *sarama.ConsumerMessage) error { return nil }

	tests := []struct {
		name    string
		cfg     *config.Kafka
		topics  []string
		handler MessageHandler
		wantMsg string
	}{
		{
			name:    "missing brokers",
			cfg:     &config.Kafka{},
			topics:  []string{TopicTranscode},
			handler: handler,
			wantMsg: "kafka brokers not configured",
		},
		{
			name:    "missing group id",
			cfg:     &config.Kafka{Brokers: []string{"b"}},
			topics:  []string{TopicTranscode},
			handler: handler,
			wantMsg: "kafka groupID required",
		},
		{
			name:    "missing topics",
			cfg:     &config.Kafka{Brokers: []string{"b"}, GroupID: "g"},
			handler: handler,
			wantMsg: "topics list required",
		},
		{
			name:    "missing handler",
			cfg:     &config.Kafka{Brokers: []string{"b"}, GroupID: "g"},
			topics:  []string{TopicTranscode},
			wantMsg: "message handler required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := zap.NewNop()

			_, err := NewKafkaConsumer(
				tt.cfg,
				tt.topics,
				tt.handler,
				NewProducerPool(logger),
				logger,
			)
			require.Error(t, err, "error = %v, want %q", err, tt.wantMsg)
			require.Equal(t, tt.wantMsg, err.Error(), "error = %v, want %q", err, tt.wantMsg)

			require.False(t, isRetryableTranscodeInitError(err), "%q must be classified as permanent", tt.wantMsg)
		})
	}
}

// TestResolveDLQTopic DLQ topic 解析优先级：dlq → transcode_dlq → "<topic>.dlq"。
func TestResolveDLQTopic(t *testing.T) {
	t.Parallel()

	explicit := &config.Kafka{Topics: map[string]string{"dlq": "my-dlq"}}
	{
		got := resolveDLQTopic(explicit, []string{TopicTranscode})
		require.Equal(t, "my-dlq", got, "explicit dlq = %q, want my-dlq", got)
	}

	alias := &config.Kafka{Topics: map[string]string{"transcode_dlq": "t-dlq"}}
	{
		got := resolveDLQTopic(alias, []string{TopicTranscode})
		require.Equal(t, "t-dlq", got, "alias dlq = %q, want t-dlq", got)
	}

	{
		got := resolveDLQTopic(&config.Kafka{}, []string{TopicTranscode})
		require.Equal(t, "transcode.dlq", got, "derived dlq = %q, want transcode.dlq", got)
	}

	{
		got := resolveDLQTopic(nil, nil)
		require.Empty(t, got, "no source dlq = %q, want empty", got)
	}
}

// TestHandleMessageRetriesThenGivesUp 消息处理失败重试 maxMessageRetryAttempts 次后返回最后一次错误。
func TestHandleMessageRetriesThenGivesUp(t *testing.T) {
	t.Parallel()

	attempts := 0
	handler := consumerGroupHandler{
		fn: func(sarama.ConsumerGroupSession, *sarama.ConsumerMessage) error {
			attempts++

			return errors.New("transient failure")
		},
	}

	err := handler.handleMessage(
		stubConsumerGroupSession{ctx: context.Background()},
		&sarama.ConsumerMessage{Topic: TopicTranscode},
	)
	require.Error(t, err, "exhausted retries must return the last error")

	require.Equal(t, maxMessageRetryAttempts+1, attempts, "attempts = %d, want %d", attempts, maxMessageRetryAttempts+1)
}

// TestHandleMessagePermanentErrorSkipsRetry 永久错误立即返回，不做退避重试。
func TestHandleMessagePermanentErrorSkipsRetry(t *testing.T) {
	t.Parallel()

	attempts := 0
	handler := consumerGroupHandler{
		fn: func(sarama.ConsumerGroupSession, *sarama.ConsumerMessage) error {
			attempts++

			return media.PermanentMessage(errors.New("bad payload"))
		},
	}

	err := handler.handleMessage(
		stubConsumerGroupSession{ctx: context.Background()},
		&sarama.ConsumerMessage{Topic: TopicTranscode},
	)

	var permanentErr *media.PermanentMessageError
	require.ErrorAs(t, err, &permanentErr, "error = %v, want PermanentMessageError", err)

	require.Equal(t, 1, attempts, "attempts = %d, want 1 (no retry for permanent failures)", attempts)
}

type stubConsumerGroupSession struct {
	ctx context.Context //nolint:containedctx // Captures the session context to verify consumer lifecycle and correlation behavior.
}

func (s stubConsumerGroupSession) Claims() map[string][]int32                  { return nil }
func (s stubConsumerGroupSession) MemberID() string                            { return "" }
func (s stubConsumerGroupSession) GenerationID() int32                         { return 0 }
func (s stubConsumerGroupSession) MarkOffset(string, int32, int64, string)     {}
func (s stubConsumerGroupSession) ResetOffset(string, int32, int64, string)    {}
func (s stubConsumerGroupSession) MarkMessage(*sarama.ConsumerMessage, string) {}
func (s stubConsumerGroupSession) Context() context.Context                    { return s.ctx }
func (s stubConsumerGroupSession) Commit()                                     {}

type stubConsumerGroupClaim struct {
	messages chan *sarama.ConsumerMessage
}

func (c stubConsumerGroupClaim) Topic() string                            { return "topic" }
func (c stubConsumerGroupClaim) Partition() int32                         { return 0 }
func (c stubConsumerGroupClaim) InitialOffset() int64                     { return 0 }
func (c stubConsumerGroupClaim) HighWaterMarkOffset() int64               { return 0 }
func (c stubConsumerGroupClaim) Messages() <-chan *sarama.ConsumerMessage { return c.messages }

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

type trackingDLQProducer struct {
	stubSyncProducer
	messages []*sarama.ProducerMessage
	fail     bool
}

func (p *trackingDLQProducer) SendMessage(m *sarama.ProducerMessage) (int32, int64, error) {
	if p.fail {
		return 0, 0, errors.New("dlq unavailable")
	}

	p.messages = append(p.messages, m)

	return 0, 0, nil
}

type trackingGroupSession struct {
	stubConsumerGroupSession
	marked []*sarama.ConsumerMessage
}

func (s *trackingGroupSession) MarkMessage(m *sarama.ConsumerMessage, _ string) {
	s.marked = append(s.marked, m)
}

func TestRetryExhaustionDeadLettersAndContinuesPartition(t *testing.T) {
	t.Parallel()

	session := &trackingGroupSession{stubConsumerGroupSession: stubConsumerGroupSession{ctx: t.Context()}}
	producer := &trackingDLQProducer{}
	messages := make(chan *sarama.ConsumerMessage, 2)
	poison := &sarama.ConsumerMessage{Topic: "source", Offset: 1}
	good := &sarama.ConsumerMessage{Topic: "source", Offset: 2}

	messages <- poison

	messages <- good

	close(messages)

	handler := consumerGroupHandler{producer: producer, dlqTopic: "source.dlq", fn: func(s sarama.ConsumerGroupSession, m *sarama.ConsumerMessage) error {
		if m.Offset == 1 {
			return context.DeadlineExceeded
		}

		s.MarkMessage(m, "")

		return nil
	}}
	{
		err := handler.ConsumeClaim(session, stubConsumerGroupClaim{messages: messages})
		require.NoError(t, err)
	}

	require.False(t, len(producer.messages) != 1 || len(session.marked) != 2 || session.marked[1] != good, "partition blocked or poison unacknowledged: dlq=%d marked=%d", len(producer.messages), len(session.marked))
}

func TestDeadLetterFailureDoesNotCommitPoison(t *testing.T) {
	t.Parallel()

	session := &trackingGroupSession{stubConsumerGroupSession: stubConsumerGroupSession{ctx: t.Context()}}

	messages := make(chan *sarama.ConsumerMessage, 1)
	messages <- &sarama.ConsumerMessage{Topic: "source"}

	close(messages)

	handler := consumerGroupHandler{producer: &trackingDLQProducer{fail: true}, dlqTopic: "source.dlq", fn: func(_ sarama.ConsumerGroupSession, _ *sarama.ConsumerMessage) error {
		return media.PermanentMessage(errors.New("invalid"))
	}}
	{
		err := handler.ConsumeClaim(session, stubConsumerGroupClaim{messages: messages})
		require.Error(t, err, "expected dlq failure")
	}

	require.Empty(t, session.marked, "poison acknowledged without durable dlq copy")
}
