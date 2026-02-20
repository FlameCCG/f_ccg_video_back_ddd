package mq

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	media "f_ccg_video_back_ddd/internal/domain/media"
	"f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

func TestShouldRecreateConsumerGroup(t *testing.T) {
	t.Parallel()

	t.Run("nil error", func(t *testing.T) {
		t.Parallel()

		if shouldRecreateConsumerGroup(nil) {
			t.Fatal("nil error should not recreate consumer group")
		}
	})

	t.Run("kafka coordinator error", func(t *testing.T) {
		t.Parallel()

		if !shouldRecreateConsumerGroup(sarama.ErrNotCoordinatorForConsumer) {
			t.Fatal("ErrNotCoordinatorForConsumer should recreate consumer group")
		}
	})

	t.Run("wrapped broker exhaustion error", func(t *testing.T) {
		t.Parallel()

		err := sarama.Wrap(sarama.ErrOutOfBrokers, errors.New("dial tcp timeout"))
		if !shouldRecreateConsumerGroup(err) {
			t.Fatal("wrapped ErrOutOfBrokers should recreate consumer group")
		}
	})

	t.Run("plain string match", func(t *testing.T) {
		t.Parallel()

		err := errors.New("kafka server: Request was for a consumer group that is not coordinated by this broker")
		if !shouldRecreateConsumerGroup(err) {
			t.Fatal("coordinator string error should recreate consumer group")
		}
	})

	t.Run("network timeout", func(t *testing.T) {
		t.Parallel()

		err := &net.OpError{Op: "read", Net: "tcp", Err: timeoutError{}}
		if !shouldRecreateConsumerGroup(err) {
			t.Fatal("network timeout should recreate consumer group")
		}
	})

	t.Run("unrelated error", func(t *testing.T) {
		t.Parallel()

		if shouldRecreateConsumerGroup(errors.New("boom")) {
			t.Fatal("unrelated error should not recreate consumer group")
		}
	})
}

func TestNewConsumerGroupConfigUsesOldestOffset(t *testing.T) {
	t.Parallel()

	cfg := &config.Kafka{
		ClientID: "test-client",
		Version:  "3.9.0",
	}

	kCfg := newConsumerGroupConfig(cfg, zap.NewNop())
	if kCfg.Consumer.Offsets.Initial != sarama.OffsetOldest {
		t.Fatalf("expected initial offset %d, got %d", sarama.OffsetOldest, kCfg.Consumer.Offsets.Initial)
	}
}

func TestResolveKafkaRebalanceTimeout(t *testing.T) {
	t.Parallel()

	if got := resolveKafkaRebalanceTimeout(nil); got != defaultKafkaRebalanceTimeout {
		t.Fatalf("expected default rebalance timeout %v, got %v", defaultKafkaRebalanceTimeout, got)
	}

	cfg := &config.Kafka{RebalanceTimeoutSec: 600}
	if got := resolveKafkaRebalanceTimeout(cfg); got != 600*time.Second {
		t.Fatalf("expected configured rebalance timeout 600s, got %v", got)
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

	if err := consumer.consumeCtx.Err(); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected consume ctx canceled, got %v", err)
	}

	if err := consumer.ShutdownContext().Err(); err != nil {
		t.Fatalf("shutdown ctx should remain active after StopConsuming, got %v", err)
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

	if err := consumer.Close(); err != nil {
		t.Fatalf("close consumer failed: %v", err)
	}

	if !errors.Is(consumer.ShutdownContext().Err(), context.Canceled) {
		t.Fatalf("expected shutdown ctx canceled after Close, got %v", consumer.ShutdownContext().Err())
	}
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
	if err != nil {
		t.Fatalf("expected canceled session error to be suppressed, got %v", err)
	}
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
			topics:  []string{"transcode"},
			handler: handler,
			wantMsg: "kafka brokers not configured",
		},
		{
			name:    "missing group id",
			cfg:     &config.Kafka{Brokers: []string{"b"}},
			topics:  []string{"transcode"},
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
			topics:  []string{"transcode"},
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
			if err == nil || err.Error() != tt.wantMsg {
				t.Fatalf("error = %v, want %q", err, tt.wantMsg)
			}

			if isRetryableTranscodeInitError(err) {
				t.Fatalf("%q must be classified as permanent", tt.wantMsg)
			}
		})
	}
}

// TestResolveDLQTopic DLQ topic 解析优先级：dlq → transcode_dlq → "<topic>.dlq"。
func TestResolveDLQTopic(t *testing.T) {
	t.Parallel()

	explicit := &config.Kafka{Topics: map[string]string{"dlq": "my-dlq"}}
	if got := resolveDLQTopic(explicit, []string{"transcode"}); got != "my-dlq" {
		t.Fatalf("explicit dlq = %q, want my-dlq", got)
	}

	alias := &config.Kafka{Topics: map[string]string{"transcode_dlq": "t-dlq"}}
	if got := resolveDLQTopic(alias, []string{"transcode"}); got != "t-dlq" {
		t.Fatalf("alias dlq = %q, want t-dlq", got)
	}

	if got := resolveDLQTopic(&config.Kafka{}, []string{"transcode"}); got != "transcode.dlq" {
		t.Fatalf("derived dlq = %q, want transcode.dlq", got)
	}

	if got := resolveDLQTopic(nil, nil); got != "" {
		t.Fatalf("no source dlq = %q, want empty", got)
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
		&sarama.ConsumerMessage{Topic: "transcode"},
	)
	if err == nil {
		t.Fatal("exhausted retries must return the last error")
	}

	if attempts != maxMessageRetryAttempts+1 {
		t.Fatalf("attempts = %d, want %d", attempts, maxMessageRetryAttempts+1)
	}
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
		&sarama.ConsumerMessage{Topic: "transcode"},
	)

	var permanentErr *media.PermanentMessageError
	if !errors.As(err, &permanentErr) {
		t.Fatalf("error = %v, want PermanentMessageError", err)
	}

	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (no retry for permanent failures)", attempts)
	}
}

type stubConsumerGroupSession struct {
	ctx context.Context
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
